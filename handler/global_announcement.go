package handler

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/pyama86/YAS3/domain/entity"
	"github.com/pyama86/YAS3/presentation/blocks"
	"github.com/slack-go/slack"
)

const globalAnnounceModalCallbackID = "global_announce_modal"

// openGlobalAnnounceModal は障害報告の入力モーダルを開く。
// trigger_idは3秒で失効するため、まずローディングモーダルを即時に開き、
// 裏でAIが下書きを生成してから views.update でフォームに差し替える。
func (h *CallbackHandler) openGlobalAnnounceModal(triggerID, channelID string) error {
	incident, err := h.repository.FindIncidentByChannel(h.ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to FindIncidentByChannel: %w", err)
	}
	if incident == nil {
		return fmt.Errorf("incident is nil")
	}

	externalID := fmt.Sprintf("global_announce_%s", channelID)

	loading := slack.ModalViewRequest{
		Type:            slack.ViewType("modal"),
		Title:           slack.NewTextBlockObject("plain_text", "📢 障害報告", false, false),
		Close:           slack.NewTextBlockObject("plain_text", "❌ キャンセル", false, false),
		Blocks:          blocks.GlobalAnnounceLoading(),
		PrivateMetadata: channelID,
		ExternalID:      externalID,
		CallbackID:      globalAnnounceModalCallbackID,
	}
	if err := h.repository.OpenView(triggerID, loading); err != nil {
		return fmt.Errorf("failed to OpenView: %w", err)
	}

	// AI生成は時間がかかるため非同期で実行し、完了後にviews.updateで差し替える
	go h.updateGlobalAnnounceModalWithDraft(channelID, externalID, incident)

	return nil
}

// updateGlobalAnnounceModalWithDraft はAIで下書きを生成し、views.updateでフォームに差し替える
func (h *CallbackHandler) updateGlobalAnnounceModalWithDraft(channelID, externalID string, incident *entity.Incident) {
	summary, occurredAt, impact := h.buildAnnounceDraft(channelID, incident)

	view := slack.ModalViewRequest{
		Type:            slack.ViewType("modal"),
		Title:           slack.NewTextBlockObject("plain_text", "📢 障害報告", false, false),
		Submit:          slack.NewTextBlockObject("plain_text", "✅ 報告する", false, false),
		Close:           slack.NewTextBlockObject("plain_text", "❌ キャンセル", false, false),
		Blocks:          blocks.GlobalAnnounceForm(summary, occurredAt, impact),
		PrivateMetadata: channelID,
		ExternalID:      externalID,
		CallbackID:      globalAnnounceModalCallbackID,
	}
	if err := h.repository.UpdateView(externalID, view); err != nil {
		slog.Error("Failed to UpdateView for global announce", slog.Any("err", err))
	}
}

// buildAnnounceDraft はAIで発生事象・発生日時・影響範囲の下書きを生成する。
// AIが未設定/失敗時はインシデント情報からのフォールバック値を返す。
func (h *CallbackHandler) buildAnnounceDraft(channelID string, incident *entity.Incident) (summary, occurredAt, impact string) {
	summary = incident.Description
	occurredAt = incident.StartedAt.Format("2006-01-02 15:04")
	impact = ""

	if h.aiRepository == nil {
		return summary, occurredAt, impact
	}

	messages, err := h.collectChannelMessages(channelID, incident)
	if err != nil {
		slog.Error("Failed to collectChannelMessages for announce draft", slog.Any("err", err))
		return summary, occurredAt, impact
	}

	var b strings.Builder
	for _, m := range messages {
		fmt.Fprintf(&b, "%s: %s\n", m.User, m.Text)
	}
	formatted := b.String()

	if s, err := h.aiRepository.Summarize(incident.Description, formatted); err == nil && strings.TrimSpace(s) != "" {
		summary = s
	}
	if oc, err := h.aiRepository.GenerateOccurredAt(incident.Description, formatted, occurredAt); err == nil && strings.TrimSpace(oc) != "" {
		occurredAt = oc
	}
	if im, err := h.aiRepository.GenerateImpact(incident.Description, formatted); err == nil && strings.TrimSpace(im) != "" {
		impact = im
	}

	return summary, occurredAt, impact
}

// submitGlobalAnnounceModal はフォーム送信を受けて global_announcement_channels のみに周知する
func (h *CallbackHandler) submitGlobalAnnounceModal(callback *slack.InteractionCallback) error {
	channelID := callback.View.PrivateMetadata
	summary := callback.View.State.Values["summary_block"]["summary_text"].Value
	occurredAt := callback.View.State.Values["occurred_block"]["occurred_text"].Value
	impact := callback.View.State.Values["impact_block"]["impact_text"].Value
	userID := callback.User.ID

	incident, err := h.repository.FindIncidentByChannel(h.ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to FindIncidentByChannel: %w", err)
	}
	if incident == nil {
		return fmt.Errorf("incident is nil")
	}

	service, err := h.repository.ServiceByID(h.ctx, incident.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to ServiceByID: %w", err)
	}

	attachment := slack.Attachment{
		Color:  urgencyColorMap[incident.Urgency],
		Blocks: slack.Blocks{BlockSet: blocks.GlobalAnnouncement(summary, occurredAt, impact, channelID, service)},
	}

	return h.announceToGlobalChannels(channelID, userID, attachment)
}

// announceToGlobalChannels は global_announcement_channels のみに通知する（手動周知用）。
// サービス固有のアナウンスチャンネルはインシデント作成時に自動通知済みのため対象外とする。
func (h *CallbackHandler) announceToGlobalChannels(channelID, userID string, attachment slack.Attachment) error {
	if h.config == nil {
		return nil
	}

	channels := h.config.GetGlobalAnnouncementChannels(h.ctx)
	if len(channels) == 0 {
		if _, _, err := h.repository.PostMessage(
			channelID,
			slack.MsgOptionText("⚠️ global_announcement_channels が設定されていません", false),
		); err != nil {
			slog.Error("Failed to post no global channels message", slog.Any("err", err))
		}
		return nil
	}

	notificationType := h.config.GetNotificationType()
	posted := make(map[string]bool)
	for _, c := range channels {
		cinfo, err := h.repository.GetChannelByName(c)
		if err != nil || cinfo == nil {
			slog.Error("failed to GetChannelByName", slog.Any("channel", c), slog.Any("err", err))
			continue
		}
		if posted[cinfo.ID] {
			continue
		}

		var msgOptions []slack.MsgOption
		if notificationText := blocks.AddNotification("", notificationType); notificationText != "" {
			msgOptions = append(msgOptions, slack.MsgOptionText(notificationText, false))
		}
		msgOptions = append(msgOptions, slack.MsgOptionAttachments(attachment))

		if _, _, err := h.repository.PostMessage(cinfo.ID, msgOptions...); err != nil {
			return fmt.Errorf("failed to post announcement to channel %s: %w", cinfo.Name, err)
		}
		posted[cinfo.ID] = true

		if _, _, err := h.repository.PostMessage(
			channelID,
			slack.MsgOptionText(fmt.Sprintf("📢 <@%s> が %s チャンネルに障害報告しました", userID, cinfo.Name), false),
		); err != nil {
			slog.Error("Failed to post notification message", slog.Any("err", err))
		}
	}
	return nil
}
