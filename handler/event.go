package handler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pyama86/YAS3/domain/repository"
	"github.com/pyama86/YAS3/presentation/blocks"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

type EventHandler struct {
	ctx        context.Context
	client     *slack.Client
	repository repository.IncidentRepositoryer
}

func NewEventHandler(ctx context.Context, client *slack.Client, repository repository.IncidentRepositoryer) *EventHandler {
	return &EventHandler{
		ctx:        ctx,
		client:     client,
		repository: repository,
	}
}

func (h *EventHandler) Handle(event *slackevents.EventsAPIInnerEvent) error {
	switch ev := event.Data.(type) {
	case *slackevents.AppMentionEvent:
		slog.Info("AppMentionEvent", "user", ev.User, "channel", ev.Channel)
		return h.handleMetionEvent(ev)
	case *slackevents.ChannelArchiveEvent:
		slog.Info("ChannelArchiveEvent", "user", ev.User, "channel", ev.Channel)
		return h.saveClosedAt(ev)
	}
	return nil
}

func (h *EventHandler) saveClosedAt(event *slackevents.ChannelArchiveEvent) error {
	channelID := event.Channel
	incident, err := h.repository.FindIncidentByChannel(h.ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to FindIncidentByChannel: %w", err)
	}
	if incident == nil {
		return nil
	}
	incident.ClosedAt = timeNow()
	err = h.repository.SaveIncident(h.ctx, incident)
	if err != nil {
		return fmt.Errorf("failed to UpdateClosedAt: %w", err)
	}
	return nil
}

func (h *EventHandler) handleMetionEvent(event *slackevents.AppMentionEvent) error {
	channelID := event.Channel

	incident, err := h.repository.FindIncidentByChannel(h.ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to FindIncidentByChannel: %w", err)
	}

	// インシデントチャンネルではなければインシデント作成
	if incident == nil {
		msgOptions := []slack.MsgOption{
			slack.MsgOptionBlocks(blocks.Opening()...),
		}

		if event.ThreadTimeStamp != "" {
			msgOptions = append(msgOptions, slack.MsgOptionTS(event.ThreadTimeStamp))
		}

		_, _, err := h.client.PostMessage(
			channelID,
			msgOptions...,
		)
		if err != nil {
			return fmt.Errorf("failed to PostEphemeral: %w", err)
		}
	} else {
		msgOptions := []slack.MsgOption{
			slack.MsgOptionBlocks(blocks.IncidentMenu()...),
		}
		if event.ThreadTimeStamp != "" {
			msgOptions = append(msgOptions, slack.MsgOptionTS(event.ThreadTimeStamp))
		}

		_, _, err := h.client.PostMessage(
			channelID,
			msgOptions...,
		)

		if err != nil {
			return fmt.Errorf("failed to PostEphemeral: %w", err)
		}
	}
	return nil
}
