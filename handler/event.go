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
	repository repository.Repository
	config     *repository.Config
}

func NewEventHandler(ctx context.Context, client *slack.Client, repository repository.Repository, config *repository.Config) *EventHandler {
	return &EventHandler{
		ctx:        ctx,
		client:     client,
		repository: repository,
		config:     config,
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

	// インシデントチャンネルではなければ紐づけメニューを表示
	if incident == nil {
		// アナウンスチャンネルかどうかをチェック
		isAnnouncementChannel, err := h.checkIfAnnouncementChannel(channelID)
		if err != nil {
			return fmt.Errorf("failed to checkIfAnnouncementChannel: %w", err)
		}

		// スレッドかチャンネルかを判定
		isThread := event.ThreadTimeStamp != ""

		// アナウンスチャンネルかつチャンネル直接の場合はインシデント作成ボタンのみ表示
		if isAnnouncementChannel && !isThread {
			msgOptions := []slack.MsgOption{
				slack.MsgOptionBlocks(
					slack.NewSectionBlock(
						slack.NewTextBlockObject(
							"mrkdwn",
							"新しいインシデントを作成する場合:",
							false,
							false,
						),
						nil,
						nil,
					),
					slack.NewActionBlock(
						"incident_create_action",
						slack.NewButtonBlockElement(
							"incident_action",
							"create_new_incident",
							slack.NewTextBlockObject("plain_text", "🚨 インシデント作成", false, false),
						).WithStyle(slack.StyleDanger),
						slack.NewButtonBlockElement(
							"cancel_action",
							"cancel_button",
							slack.NewTextBlockObject("plain_text", "❌ キャンセル", false, false),
						),
					),
				),
			}

			_, _, err := h.client.PostMessage(
				channelID,
				msgOptions...,
			)
			if err != nil {
				return fmt.Errorf("failed to PostMessage: %w", err)
			}
			return nil
		}

		// それ以外（通常チャンネルまたはアナウンスチャンネルのスレッド）は紐づけメニューを表示
		// 現在の紐づけ状態をチェック
		var checkThreadTS string
		if isThread {
			checkThreadTS = event.ThreadTimeStamp
		}
		isLinked, err := h.checkIfLinked(channelID, checkThreadTS)
		if err != nil {
			return fmt.Errorf("failed to checkIfLinked: %w", err)
		}

		msgOptions := []slack.MsgOption{
			slack.MsgOptionBlocks(blocks.LinkIncidentMenu(isThread, isLinked)...),
		}

		if isThread {
			msgOptions = append(msgOptions, slack.MsgOptionTS(event.ThreadTimeStamp))
		}

		_, _, err = h.client.PostMessage(
			channelID,
			msgOptions...,
		)
		if err != nil {
			return fmt.Errorf("failed to PostMessage: %w", err)
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

// 指定されたチャンネル/スレッドが既にインシデントに紐づけられているかチェック
func (h *EventHandler) checkIfLinked(channelID, threadTS string) (bool, error) {
	slog.Info("checkIfLinked", slog.Any("channelID", channelID), slog.Any("threadTS", threadTS))

	// 全てのアクティブなインシデントから該当の紐づけを検索
	incidents, err := h.repository.ActiveIncidents(h.ctx)
	if err != nil {
		return false, fmt.Errorf("failed to ActiveIncidents: %w", err)
	}

	for _, incident := range incidents {
		slog.Info("checking incident", slog.Any("incidentChannelID", incident.ChannelID), slog.Any("linkedChannels", len(incident.LinkedChannels)))
		for _, linked := range incident.LinkedChannels {
			slog.Info("checking linked", slog.Any("linkedChannelID", linked.ChannelID), slog.Any("linkedThreadTS", linked.ThreadTS))
			if linked.ChannelID == channelID && linked.ThreadTS == threadTS {
				slog.Info("found match - already linked")
				return true, nil
			}
		}
	}

	slog.Info("no match found - not linked")
	return false, nil
}

// 指定されたチャンネルがアナウンスチャンネルかどうかをチェック
func (h *EventHandler) checkIfAnnouncementChannel(channelID string) (bool, error) {
	// チャンネル情報を取得
	channel, err := h.repository.GetChannelByID(channelID)
	if err != nil {
		return false, fmt.Errorf("failed to GetChannelByID: %w", err)
	}
	if channel == nil {
		return false, nil
	}

	// 全サービスのアナウンスチャンネルを確認
	services, err := h.repository.Services(h.ctx)
	if err != nil {
		return false, fmt.Errorf("failed to Services: %w", err)
	}

	for _, service := range services {
		for _, announcementChannel := range service.AnnouncementChannels {
			if announcementChannel == channel.Name {
				return true, nil
			}
		}
	}

	// グローバルアナウンスチャンネルも確認
	if h.config != nil {
		for _, globalChannel := range h.config.GetGlobalAnnouncementChannels(h.ctx) {
			if globalChannel == channel.Name {
				return true, nil
			}
		}
	}

	return false, nil
}
