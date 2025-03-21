package handler

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/pyama86/YAS3/domain/entity"
	"github.com/pyama86/YAS3/domain/repository"
	"github.com/pyama86/YAS3/presentation/blocks"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

type Handler interface {
	Handle(event slackevents.EventsAPIInnerEvent) error
}

func Handle(ctx context.Context, configPath string) error {
	webApi := slack.New(
		os.Getenv("SLACK_BOT_TOKEN"),
		slack.OptionAppLevelToken(os.Getenv("SLACK_APP_TOKEN")),
	)
	socketMode := socketmode.New(
		webApi,
	)
	authTest, authTestErr := webApi.AuthTest()
	if authTestErr != nil {
		fmt.Fprintf(os.Stderr, "SLACK_BOT_TOKEN is invalid: %v\n", authTestErr)
		os.Exit(1)
	}
	botID := authTest.UserID
	workSpaceURL := authTest.URL
	slog.Info("Bot ID", slog.String("bot_id", botID))

	dynamoRepository, err := repository.NewDynamoDBRepository()
	if err != nil {
		return err
	}

	cfgRepository, err := repository.NewConfigRepository(configPath)
	if err != nil {
		return err
	}

	slackRepository := repository.NewSlackRepository(webApi)

	airtableRepository, err := repository.NewAIRepository()
	if err != nil {
		return err
	}

	repo := repository.NewRepository(dynamoRepository, cfgRepository, cfgRepository)
	if err != nil {
		return err
	}

	var postmortemExporter repository.PostMortemExporter
	if os.Getenv("CONFLUENCE_USERNAME") != "" && os.Getenv("CONFLUENCE_PASSWORD") != "" && cfgRepository.Confluence.Domain != "" {
		r, err := repository.NewConfluenceRepository(
			cfgRepository.Confluence.Domain,
			os.Getenv("CONFLUENCE_USERNAME"),
			os.Getenv("CONFLUENCE_PASSWORD"),
			cfgRepository.Confluence.Space,
			cfgRepository.Confluence.AncestorID,
		)
		if err != nil {
			return err
		}
		postmortemExporter = r
	}

	eventHandler := NewEventHandler(
		ctx,
		webApi,
		repo,
	)

	callbackHandler := NewCallbackHandler(
		ctx,
		repo,
		slackRepository,
		airtableRepository,
		postmortemExporter,
		workSpaceURL,
	)

	// 15分ごとにインシデントチャンネルに通知を行う
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			incidents, err := repo.ActiveIncidents(ctx)
			if err != nil {
				continue
			}
			for _, incident := range incidents {
				if !incident.DisableTimer {
					if err := timeKeeperMessage(webApi, &incident, slackRepository); err != nil {
						slog.Error("Failed to send time keeper message", slog.Any("err", err))
					}
				}
			}
		}
	}()

	go func() {
		for envelope := range socketMode.Events {
			switch envelope.Type {
			case socketmode.EventTypeEventsAPI:
				socketMode.Ack(*envelope.Request)
				eventPayload, ok := envelope.Data.(slackevents.EventsAPIEvent)
				if !ok {
					slog.Error("Failed to cast to EventsAPIEvent")
					continue
				}

				switch eventPayload.Type {
				case slackevents.CallbackEvent:
					innerEvent := eventPayload.InnerEvent
					if err := eventHandler.Handle(&innerEvent); err != nil {
						slog.Error("Failed to handle event", slog.Any("err", err))
					}
				}
			case socketmode.EventTypeInteractive:
				socketMode.Ack(*envelope.Request)
				callback, ok := envelope.Data.(slack.InteractionCallback)
				if !ok {
					slog.Error("Failed to cast to InteractionCallback")
					continue
				}
				if err := callbackHandler.Handle(&callback); err != nil {
					slog.Error("Failed to handle callback", slog.Any("err", err))
				}
			}
		}
	}()

	return socketMode.Run()
}

func timeKeeperMessage(client *slack.Client, incident *entity.Incident, slackRepository *repository.SlackRepository) error {
	channelID := incident.ChannelID
	channel, err := slackRepository.GetChannelByID(channelID)
	if err != nil {
		if err == repository.ErrSlackNotFound {
			return nil
		}
		return fmt.Errorf("failed to get channel %s: %w", channelID, err)
	}
	if channel.IsArchived {
		return nil
	}

	elapsed := time.Since(incident.StartedAt)
	hours := int(elapsed.Hours())
	minutes := int(elapsed.Minutes()) % 60
	elapsedStr := fmt.Sprintf("%d時間%d分", hours, minutes)

	// 15分ごとのチェックポイントの案内
	_, _, err = client.PostMessage(
		channelID,
		slack.MsgOptionBlocks(blocks.CheckPoint(elapsedStr)...),
	)
	if err != nil {
		return fmt.Errorf("failed to post time keeper message %s: %w", channel.Name, err)
	}
	return nil
}
