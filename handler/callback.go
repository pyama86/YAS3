package handler

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pyama86/YAS3/domain/entity"
	"github.com/pyama86/YAS3/domain/repository"
	"github.com/pyama86/YAS3/presentation/blocks"
	"github.com/pyama86/YAS3/presentation/postmortem"
	"github.com/slack-go/slack"
)

func timeNow() time.Time {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		loc = time.UTC
	}
	return time.Now().In(loc)
}

type CallbackHandler struct {
	ctx                context.Context
	repository         repository.Repository
	workSpaceURL       string
	aiRepository       *repository.AIRepository
	postmortemExporter repository.PostMortemRepositoryer
	config             *repository.Config
}

var urgencyColorMap = map[string]string{
	"none":     "#36a64f",
	"warning":  "#f2c744",
	"error":    "#f2c744",
	"critical": "#ff0000",
}

func NewCallbackHandler(
	ctx context.Context,
	repository repository.Repository,
	workSpaceURL string,
	aiRepository *repository.AIRepository,
	postmortemExporter repository.PostMortemRepositoryer,
	config *repository.Config,
) *CallbackHandler {
	return &CallbackHandler{
		ctx:                ctx,
		repository:         repository,
		aiRepository:       aiRepository,
		workSpaceURL:       workSpaceURL,
		postmortemExporter: postmortemExporter,
		config:             config,
	}
}

func (h *CallbackHandler) Handle(callback *slack.InteractionCallback) error {
	switch callback.Type {
	case slack.InteractionTypeBlockActions:
		if len(callback.ActionCallback.BlockActions) < 1 {
			return fmt.Errorf("block_actions is empty")
		}
		action := callback.ActionCallback.BlockActions[0]

		switch action.ActionID {
		case "incident_action":
			if err := h.openIncidentModal(callback.TriggerID, callback.Channel.ID); err != nil {
				return fmt.Errorf("openIncidentModal failed: %w", err)
			}

			h.repository.UpdateMessage(
				callback.Channel.ID,
				callback.Message.Timestamp,
				slack.MsgOptionBlocks(blocks.UserIsTyping(callback.User.ID)...),
			)
		case "handler_button":
			h.repository.DeleteMessage(
				callback.Channel.ID,
				callback.Message.Timestamp,
			)
			if err := h.submitHandler(callback.User.ID, callback.Channel.ID); err != nil {
				return fmt.Errorf("submitHandler failed: %w", err)
			}
		case "incident_level_button":
			h.repository.DeleteMessage(
				callback.Channel.ID,
				callback.Message.Timestamp,
			)

			slog.Info("incident_level_options", slog.Any("channelID", callback.Channel.ID), slog.Any("value", callback.ActionCallback.BlockActions[0].Value))

			if err := h.setIncidentLevel(callback.Channel.ID, callback.User.ID, callback.ActionCallback.BlockActions[0].Value); err != nil {
				return fmt.Errorf("setIncidentLevel failed: %w", err)
			}
		case "postmortem_action":
			h.repository.UpdateMessage(
				callback.Channel.ID,
				callback.Message.Timestamp,
				slack.MsgOptionText("📝 ポストモーテムを作成中...", false),
			)
			if err := h.createPostMortem(callback.Channel, callback.User); err != nil {
				return fmt.Errorf("createPostMortem failed: %w", err)
			}
		case "progress_summary_action":
			h.repository.UpdateMessage(
				callback.Channel.ID,
				callback.Message.Timestamp,
				slack.MsgOptionBlocks(blocks.ProgressSummaryLoading()...),
			)
			if err := h.createProgressSummary(callback.Channel, callback.User); err != nil {
				return fmt.Errorf("createProgressSummary failed: %w", err)
			}
		case "report_post_action":
			if err := h.postToReportChannel(callback.Channel, callback.User, callback.Message); err != nil {
				return fmt.Errorf("postToReportChannel failed: %w", err)
			}
			// 報告後にボタンを削除するため、メッセージを更新
			if err := h.removeReportButtonFromMessage(callback.Channel.ID, callback.Message); err != nil {
				return fmt.Errorf("failed to removeReportButtonFromMessage: %w", err)
			}
		// 確認フォームのボタン処理
		case "progress_summary_execute":
			// 確認メッセージを削除
			h.repository.DeleteMessage(callback.Channel.ID, callback.Message.Timestamp)
			// 作成中メッセージを新規投稿
			_, loadingMsgTS, err := h.repository.PostMessage(
				callback.Channel.ID,
				slack.MsgOptionBlocks(blocks.ProgressSummaryLoading()...),
			)
			if err != nil {
				return fmt.Errorf("failed to post loading message: %w", err)
			}
			// サマリ作成処理を実行し、作成中メッセージを更新
			if err := h.createProgressSummaryWithUpdate(callback.Channel, callback.User, loadingMsgTS); err != nil {
				return fmt.Errorf("createProgressSummary failed: %w", err)
			}
		case "progress_summary_cancel":
			// 確認メッセージを削除
			h.repository.DeleteMessage(callback.Channel.ID, callback.Message.Timestamp)
		case "recovery_execute":
			// 確認メッセージを削除
			h.repository.DeleteMessage(callback.Channel.ID, callback.Message.Timestamp)
			// 復旧処理を実行
			if err := h.recoveryIncident(callback.User.ID, callback.Channel.ID); err != nil {
				return fmt.Errorf("recoveryIncident failed: %w", err)
			}
		case "recovery_cancel":
			// 確認メッセージを削除
			h.repository.DeleteMessage(callback.Channel.ID, callback.Message.Timestamp)
		case "timekeeper_stop_execute":
			// 確認メッセージを削除
			h.repository.DeleteMessage(callback.Channel.ID, callback.Message.Timestamp)
			// タイムキーパー停止処理を実行
			if err := h.stopTimeKeeper(callback.Channel.ID, callback.User.ID); err != nil {
				return fmt.Errorf("stopTimeKeeper failed: %w", err)
			}
		case "timekeeper_stop_cancel":
			// 確認メッセージを削除
			h.repository.DeleteMessage(callback.Channel.ID, callback.Message.Timestamp)
		case "in_channel_options":
			if action.BlockID == "keeper_action" {
				currentBlocks := callback.Message.Blocks.BlockSet
				if len(currentBlocks) > 0 {
					currentBlocks = currentBlocks[:len(currentBlocks)-1]
				}

				h.repository.UpdateMessage(
					callback.Channel.ID,
					callback.Message.Timestamp,
					slack.MsgOptionBlocks(currentBlocks...),
				)

			} else {
				h.repository.DeleteMessage(
					callback.Channel.ID,
					callback.Message.Timestamp,
				)
			}
			switch callback.ActionCallback.BlockActions[0].SelectedOption.Value {
			case "recovery_incident":
				slog.Info("recovery_incident", slog.Any("channelID", callback.Channel.ID))
				// 確認フォームを表示
				_, _, err := h.repository.PostMessage(
					callback.Channel.ID,
					slack.MsgOptionBlocks(blocks.RecoveryConfirmation()...),
				)
				if err != nil {
					slog.Error("Failed to post recovery confirmation", slog.Any("err", err))
				}

			case "reopen_incident":
				slog.Info("reopen_incident", slog.Any("channelID", callback.Channel.ID))
				if err := h.reopenIncident(callback.User.ID, callback.Channel.ID); err != nil {
					return fmt.Errorf("reopenIncident failed: %w", err)
				}

			case "stop_timekeeper":
				slog.Info("stop_timekeeper", slog.Any("channelID", callback.Channel.ID))
				// 確認フォームを表示
				_, _, err := h.repository.PostMessage(
					callback.Channel.ID,
					slack.MsgOptionBlocks(blocks.TimekeeperStopConfirmation()...),
				)
				if err != nil {
					slog.Error("Failed to post timekeeper stop confirmation", slog.Any("err", err))
				}
			case "set_incident_level":
				slog.Info("set_incident_level", slog.Any("channelID", callback.Channel.ID))
				h.showIncidentLevelButtons(callback.Channel.ID)
			case "edit_incident_summary":
				slog.Info("edit_incident_summary", slog.Any("channelID", callback.Channel.ID))
				if err := h.openEditSummaryModal(callback.TriggerID, callback.Channel.ID); err != nil {
					return fmt.Errorf("openEditSummaryModal failed: %w", err)
				}
			case "create_postmortem":
				slog.Info("create_postmortem", slog.Any("channelID", callback.Channel.ID))
				if err := h.showPostMortemButton(callback.Channel.ID); err != nil {
					return fmt.Errorf("showPostMortemButton failed: %w", err)
				}
			case "create_progress_summary":
				slog.Info("create_progress_summary", slog.Any("channelID", callback.Channel.ID))
				// 確認フォームを表示
				_, _, err := h.repository.PostMessage(
					callback.Channel.ID,
					slack.MsgOptionBlocks(blocks.ProgressSummaryConfirmation()...),
				)
				if err != nil {
					slog.Error("Failed to post progress summary confirmation", slog.Any("err", err))
				}
			}

		case "link_incident_options":
			h.repository.DeleteMessage(
				callback.Channel.ID,
				callback.Message.Timestamp,
			)
			switch callback.ActionCallback.BlockActions[0].SelectedOption.Value {
			case "list_open_incidents":
				slog.Info("list_open_incidents", slog.Any("channelID", callback.Channel.ID))
				if err := h.listOpenIncidents(callback.Channel.ID, ""); err != nil {
					return fmt.Errorf("listOpenIncidents failed: %w", err)
				}
			case "link_to_incident":
				slog.Info("link_to_incident", slog.Any("channelID", callback.Channel.ID))
				if err := h.showActiveIncidentsList(callback); err != nil {
					return fmt.Errorf("showActiveIncidentsList failed: %w", err)
				}
			case "unlink_from_incident":
				slog.Info("unlink_from_incident", slog.Any("channelID", callback.Channel.ID))
				if err := h.unlinkFromIncident(callback); err != nil {
					return fmt.Errorf("unlinkFromIncident failed: %w", err)
				}
			}
		case "cancel_action":
			// キャンセルボタンが押された場合、メッセージを削除してキャンセル通知を表示
			h.repository.DeleteMessage(
				callback.Channel.ID,
				callback.Message.Timestamp,
			)

			// キャンセル通知メッセージを投稿
			msgOptions := []slack.MsgOption{
				slack.MsgOptionText("❌ キャンセルしました", false),
			}

			// スレッド内でキャンセルした場合はスレッドに返信
			if callback.Message.ThreadTimestamp != "" {
				msgOptions = append(msgOptions, slack.MsgOptionTS(callback.Message.ThreadTimestamp))
			}

			_, _, err := h.repository.PostMessage(
				callback.Channel.ID,
				msgOptions...,
			)
			if err != nil {
				slog.Error("Failed to post cancel message", slog.Any("err", err))
			}

		}
	case slack.InteractionTypeViewSubmission:
		switch callback.View.CallbackID {
		case "incident_modal":
			if err := h.submitIncidentModal(callback); err != nil {
				return fmt.Errorf("submitIncidentModal failed: %w", err)
			}
		case "edit_summary_modal":
			if err := h.submitEditSummaryModal(callback); err != nil {
				return fmt.Errorf("submitEditSummaryModal failed: %w", err)
			}
		case "link_incident_modal":
			if err := h.submitLinkIncidentModal(callback); err != nil {
				return fmt.Errorf("submitLinkIncidentModal failed: %w", err)
			}
		}
	}
	return nil
}

// インシデントハンドラーが応募されたら、保存してハンドラに必要なことを通知する
func (h *CallbackHandler) submitHandler(userID, channelID string) error {
	incident, err := h.repository.FindIncidentByChannel(h.ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to FindIncidentByChannel: %w", err)
	}

	if incident == nil {
		return fmt.Errorf("incident is nil")
	}

	// インシデントにハンドラを保存する
	incident.HandlerUserID = userID
	if err := h.repository.SaveIncident(h.ctx, incident); err != nil {
		return fmt.Errorf("failed to SaveIncident: %w", err)
	}

	_, _, err = h.repository.PostMessage(
		channelID,
		slack.MsgOptionBlocks(blocks.AcceptIncidentHandler(userID)...),
	)
	if err != nil {
		return fmt.Errorf("failed to post accept incident handler message: %w", err)
	}

	return nil
}

func (h *CallbackHandler) openIncidentModal(triggerID, channelID string) error {
	titleText := slack.NewTextBlockObject("plain_text", "🚨 インシデントチャンネル作成", false, false)
	submitText := slack.NewTextBlockObject("plain_text", "✅ 作成", false, false)
	closeText := slack.NewTextBlockObject("plain_text", "❌ キャンセル", false, false)

	services, err := h.repository.Services(h.ctx)
	if err != nil {
		return err
	}

	view := slack.ModalViewRequest{
		Type:            slack.ViewType("modal"),
		Title:           titleText,
		CallbackID:      "incident_modal",
		Submit:          submitText,
		Close:           closeText,
		Blocks:          blocks.CreateIncident(services),
		PrivateMetadata: channelID,
	}

	err = h.repository.OpenView(triggerID, view)
	if err != nil {
		return err
	}

	return err
}

func (h *CallbackHandler) submitIncidentModal(callback *slack.InteractionCallback) error {
	serviceID := callback.View.State.Values["service_block"]["service_select"].SelectedOption.Value
	summaryText := callback.View.State.Values["incident_summary_block"]["summary_text"].Value
	urgency := callback.View.State.Values["urgency_block"]["urgency_select"].SelectedOption.Value
	userID := callback.User.ID
	originalChannelID := callback.View.PrivateMetadata

	slog.Info("submitIncidentModal", slog.Any("serviceID", serviceID), slog.Any("summary_text", summaryText), slog.Any("urgency", urgency))

	// チャンネル作成
	num, err := strconv.Atoi(serviceID)
	if err != nil {
		return fmt.Errorf("failed to strconv.Atoi: %w", err)
	}

	service, err := h.repository.ServiceByID(h.ctx, num)
	if err != nil {
		return fmt.Errorf("failed to ServiceByID: %w", err)
	}

	prefix := ""
	if h.config != nil && h.config.ChannelPrefix != "" {
		prefix = h.config.ChannelPrefix
	}
	channelName := fmt.Sprintf("%s%s-%s", prefix, service.Name, timeNow().Format("2006-01-02"))

	slog.Info("get_channel_by_name", slog.Any("channelName", channelName))
	// すでに存在する場合はユニークな名前にする
	c, err := h.repository.GetChannelByName(channelName)
	if err != nil && err != repository.ErrSlackNotFound {
		return fmt.Errorf("failed to GetChannelByID: %w", err)
	}
	if c != nil {
		channelName = fmt.Sprintf("%s-%02d", channelName, timeNow().Unix()%100)
	}
	slog.Info("create_conversation", slog.Any("channelName", channelName))
	channel, err := h.repository.CreateConversation(slack.CreateConversationParams{
		ChannelName: channelName,
	})

	if err != nil {
		_, _, postErr := h.repository.PostMessage(
			callback.Channel.ID,
			slack.MsgOptionText(fmt.Sprintf("❌ チャンネルの作成に失敗しました:%s", err), false),
		)
		if postErr != nil {
			slog.Error("Failed to post channel creation error message", slog.Any("err", postErr))
		}

		return fmt.Errorf("failed to CreateConversation: %w", err)
	}
	h.repository.FlushChannelCache()
	// インシデントを保存する
	incident := &entity.Incident{
		ChannelID:     channel.ID,
		ServiceID:     num,
		Description:   summaryText,
		HandlerUserID: userID,
		Urgency:       urgency,
		Level:         0,
		CreatedUserID: userID,
		StartedAt:     timeNow(),
	}
	slog.Info("save_incident", slog.Any("incident", incident))
	if err := h.repository.SaveIncident(h.ctx, incident); err != nil {
		return fmt.Errorf("failed to SaveIncident: %w", err)
	}

	urgencyText, ok := blocks.UrgencyMap[urgency]
	if !ok {
		return fmt.Errorf("invalid urgency: %s", urgency)
	}

	topic := fmt.Sprintf("サービス名:%s 緊急度:%s 事象内容:%s", service.Name, urgencyText, summaryText)
	slog.Info("set_topic_of_conversation", slog.Any("topic", topic))
	err = h.repository.SetTopicOfConversation(channel.ID, topic)
	if err != nil {
		return fmt.Errorf("failed to SetPurposeOfConversation: %w", err)
	}
	var members []string
	errMembers := []string{}
	slog.Info("get incident_team_members", slog.Int("count", len(service.IncidentTeamMembers)))
	for _, member := range service.IncidentTeamMembers {
		memberIDs, err := h.repository.GetMemberIDs(member)
		if err != nil {
			if err == repository.ErrSlackNotFound {
				slog.Error("failed to GetMemberIDs", slog.Any("err", err), slog.Any("member", member))
				errMembers = append(errMembers, member)
				continue
			}
		}
		members = append(members, memberIDs...)
	}

	if len(members) > 0 {
		slog.Info("invite_users_to_conversation", slog.Any("members", members))
		err = h.repository.InviteUsersToConversation(channel.ID, members...)
		if err != nil {
			return fmt.Errorf("failed to InviteUsersToConversation: %w", err)
		}

		_, _, err = h.repository.PostMessage(
			channel.ID,
			slack.MsgOptionBlocks(blocks.InviteMembers(service)...),
		)
		if err != nil {
			slog.Error("Failed to post invite members message", slog.Any("err", err))
		}
	}

	if len(errMembers) > 0 {
		_, _, err := h.repository.PostMessage(
			channel.ID,
			slack.MsgOptionText(fmt.Sprintf("❌ チームメンバーの取得に失敗しました:%s", strings.Join(errMembers, ",")), false),
		)
		if err != nil {
			slog.Error("Failed to post team member error message", slog.Any("err", err))
		}
	}

	attachment := slack.Attachment{
		Color:  urgencyColorMap[urgency],
		Blocks: slack.Blocks{BlockSet: blocks.IncidentCreated(summaryText, urgencyText, channel.ID, service)},
	}
	_, _, err = h.repository.PostMessage(
		channel.ID,
		slack.MsgOptionAttachments(attachment),
	)
	if err != nil {
		slog.Error("Failed to post incident created attachment", slog.Any("err", err))
	}

	// 共有チャンネルにお知らせを投稿
	if err := h.broadCastAnnouncement(channel.ID, attachment, service, true); err != nil {
		return fmt.Errorf("failed to broadCastAnnouncement: %w", err)
	}

	_, _, err = h.repository.PostMessage(
		channel.ID,
		slack.MsgOptionBlocks(blocks.IncidentReportRequest(userID)...),
	)
	if err != nil {
		return fmt.Errorf("failed to PostMessage: %w", err)
	}

	_, _, err = h.repository.PostMessage(
		channel.ID,
		slack.MsgOptionBlocks(blocks.HandlerRecruitmentMessage()...),
	)
	if err != nil {
		slog.Error("Failed to post handler recruitment message", slog.Any("err", err))
	}

	// 元のチャンネルにインシデントチャンネルへの移動案内を送信
	if originalChannelID != "" && originalChannelID != channel.ID {
		moveMessage := fmt.Sprintf("🚨 インシデント対応は <#%s> で行います。関係者の方はそちらのチャンネルにご参加ください。", channel.ID)
		_, _, err := h.repository.PostMessage(
			originalChannelID,
			slack.MsgOptionText(moveMessage, false),
		)
		if err != nil {
			slog.Error("Failed to post move message", slog.Any("err", err))
		}
	}

	return nil
}

// 障害が復旧したらトピックを変更して、各所に通知する
func (h *CallbackHandler) recoveryIncident(userID, channelID string) error {
	incident, err := h.repository.FindIncidentByChannel(h.ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to FindIncidentByChannel: %w", err)
	}

	if incident == nil {
		return fmt.Errorf("incident is nil")
	}
	if !incident.RecoveredAt.IsZero() {
		_, _, err := h.repository.PostMessage(
			channelID,
			slack.MsgOptionBlocks(blocks.AlreadyRecovered()...),
		)
		if err != nil {
			slog.Error("Failed to post already recovered message", slog.Any("err", err))
		}
		return nil
	}

	incident.RecoveredAt = timeNow()
	incident.RecoveredUserID = userID
	incident.DisableTimer = true
	if err := h.repository.SaveIncident(h.ctx, incident); err != nil {
		return fmt.Errorf("failed to SaveIncident: %w", err)
	}

	service, err := h.repository.ServiceByID(h.ctx, incident.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to ServiceByID: %w", err)
	}

	channel, err := h.repository.GetChannelByID(channelID)
	if err != nil {
		return fmt.Errorf("failed to GetChannelByID: %w", err)
	}

	topic := fmt.Sprintf("【復旧】%s", channel.Topic.Value)
	err = h.repository.SetTopicOfConversation(channel.ID, topic)
	if err != nil {
		return fmt.Errorf("failed to SetPurposeOfConversation: %w", err)
	}

	// 通知タイプを取得
	notificationType := "here"
	if h.config != nil {
		notificationType = h.config.GetNotificationType()
	}

	attachment := slack.Attachment{
		Color:  "#36a64f",
		Blocks: slack.Blocks{BlockSet: blocks.IncidentRecovered(userID, incident.HandlerUserID, notificationType)},
	}

	_, _, err = h.repository.PostMessage(
		channelID,
		slack.MsgOptionAttachments(attachment),
	)
	if err != nil {
		slog.Error("Failed to post incident recovered message", slog.Any("err", err))
	}

	incidentLevel, err := h.repository.IncidentLevelByLevel(h.ctx, incident.Level)
	if err != nil {
		return fmt.Errorf("failed to IncidentLevelByLevel: %w", err)
	}

	attachment = slack.Attachment{
		Color: "#36a64f",
		Blocks: slack.Blocks{BlockSet: blocks.IncidentRecoverdAnnounce(
			incident.Description,
			incidentLevel.Description,
			channel.ID,
			service,
		)},
	}

	if err := h.broadCastAnnouncement(channelID, attachment, service, false); err != nil {
		return fmt.Errorf("failed to broadCastAnnouncement: %w", err)
	}

	return nil
}

// タイムキーパーを停止する
func (h *CallbackHandler) stopTimeKeeper(channelID, userID string) error {
	incident, err := h.repository.FindIncidentByChannel(h.ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to FindIncidentByChannel: %w", err)
	}

	if incident == nil {
		return fmt.Errorf("incident is nil")
	}

	incident.DisableTimer = true
	if err := h.repository.SaveIncident(h.ctx, incident); err != nil {
		return fmt.Errorf("failed to SaveIncident: %w", err)
	}

	_, _, err = h.repository.PostMessage(
		channelID,
		slack.MsgOptionBlocks(blocks.TimeKeeperStopped(userID)...),
	)
	if err != nil {
		slog.Error("Failed to post timekeeper stopped message", slog.Any("err", err))
	}
	return nil
}

// インシデントレベルを変更し、通知する
func (h *CallbackHandler) setIncidentLevel(channelID, userID, level string) error {
	incident, err := h.repository.FindIncidentByChannel(h.ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to FindIncidentByChannel: %w", err)
	}

	if incident == nil {
		return fmt.Errorf("incident is nil")
	}

	levelInt, err := strconv.Atoi(level)
	if err != nil {
		return fmt.Errorf("failed to strconv.Atoi: %w", err)
	}
	incident.Level = levelInt
	if err := h.repository.SaveIncident(h.ctx, incident); err != nil {
		return fmt.Errorf("failed to SaveIncident: %w", err)
	}

	description := "サービスに影響なし"
	levels := h.repository.IncidentLevels(h.ctx)
	for _, l := range levels {
		if l.Level == levelInt {
			description = l.Description
			break
		}
	}

	service, err := h.repository.ServiceByID(h.ctx, incident.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to ServiceByID: %w", err)
	}

	color := "#36a64f"
	if levelInt > 0 && incident.RecoveredAt.IsZero() {
		color = "#f2c744"
	}
	attachment := slack.Attachment{
		Color: color,
		Blocks: slack.Blocks{
			BlockSet: blocks.IncidentLevelUpdated(
				incident.Description,
				description,
				channelID,
				service,
				!incident.RecoveredAt.IsZero(),
			),
		},
	}

	// 通知タイプを取得
	notificationType := "here"
	if h.config != nil {
		notificationType = h.config.GetNotificationType()
	}

	_, _, err = h.repository.PostMessage(
		channelID,
		slack.MsgOptionBlocks(blocks.IncidentLevelChanged(userID, description, notificationType)...),
	)
	if err != nil {
		slog.Error("Failed to post incident level changed message", slog.Any("err", err))
	}

	if err := h.broadCastAnnouncement(channelID, attachment, service, false); err != nil {
		return fmt.Errorf("failed to broadCastAnnouncement: %w", err)
	}

	return nil
}

// インシデントレベルの選択肢を提供する
func (h *CallbackHandler) showIncidentLevelButtons(channelID string) {
	levels := h.repository.IncidentLevels(h.ctx)
	_, _, err := h.repository.PostMessage(
		channelID,
		slack.MsgOptionBlocks(blocks.IncidentLevelButtons(levels)...),
	)
	if err != nil {
		slog.Error("Failed to post incident level buttons", slog.Any("err", err))
	}
}

// ピンを打つアナウンスをして、ボタンを表示する
func (h *CallbackHandler) showPostMortemButton(channelID string) error {
	incident, err := h.repository.FindIncidentByChannel(h.ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to FindIncidentByChannel: %w", err)
	}

	if incident.RecoveredAt.IsZero() {
		_, _, err := h.repository.PostMessage(
			channelID,
			slack.MsgOptionText("⛔️まだインシデントが復旧していません", false),
		)
		if err != nil {
			slog.Error("Failed to post incident not recovered message", slog.Any("err", err))
		}
		return nil
	}

	_, _, err = h.repository.PostMessage(
		channelID,
		slack.MsgOptionBlocks(blocks.PostMortemButton()...),
	)
	if err != nil {
		slog.Error("Failed to post postmortem button", slog.Any("err", err))
	}
	return nil
}

// AIを活用して、ポストモーテムを作成する
func (h *CallbackHandler) createPostMortem(channel slack.Channel, user slack.User) error {
	incident, err := h.repository.FindIncidentByChannel(h.ctx, channel.ID)
	if err != nil {
		return fmt.Errorf("failed to FindIncidentByChannel: %w", err)
	}

	if incident.PostMortemURL != "" {
		_, _, err := h.repository.PostMessage(
			channel.ID,
			slack.MsgOptionText("⛔️ポストモーテムは既に作成されています", false),
		)
		if err != nil {
			slog.Error("Failed to post postmortem exists message", slog.Any("err", err))
		}
		return nil
	}

	createdAt := incident.StartedAt
	recoveredAt := incident.RecoveredAt

	author := h.repository.GetUserPreferredName(&user)

	createdUser, err := h.repository.GetUserByID(incident.CreatedUserID)
	if err != nil {
		return fmt.Errorf("failed to GetUserByID: %w", err)
	}

	recoveredUser, err := h.repository.GetUserByID(incident.RecoveredUserID)
	if err != nil {
		return fmt.Errorf("failed to GetUserByID: %w", err)
	}

	pinnedMessages, err := h.repository.GetPinnedMessages(channel.ID)
	if err != nil {
		return fmt.Errorf("failed to GetPinnedMessages: %w", err)
	}

	formattedMessages := fmt.Sprintf("- %s %sさんがインシデントチャンネルを作成しました\n", createdAt.Format("2006-01-02 15:04:05"), h.repository.GetUserPreferredName(createdUser))
	// - yyyy-MM-dd HH:mm:ss messageの形式でメッセージを取得

	type message struct {
		ts   time.Time
		text string
		user string
	}

	var messages []message
	var userCache = map[string]string{}
	for _, m := range pinnedMessages {
		ts, err := parseSlackTimestamp(m.Timestamp)
		if err != nil {
			return fmt.Errorf("failed to parseSlackTimestamp: %w", err)
		}
		if userCache[m.User] == "" {
			user, err := h.repository.GetUserByID(m.User)
			if err != nil {
				return fmt.Errorf("failed to GetUserByID: %w", err)
			}
			userCache[m.User] = h.repository.GetUserPreferredName(user)
		}
		messages = append(messages, message{
			ts:   ts,
			text: m.Text,
			user: userCache[m.User],
		})
	}

	// messageを時系列順に並び替え
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].ts.Before(messages[j].ts)
	})

	for _, m := range messages {
		formattedMessages += fmt.Sprintf("- %s %s:%s\n", m.ts.Format("2006-01-02 15:04:05"), m.user, m.text)
	}
	formattedMessages += fmt.Sprintf("- %s %sさんがインシデントを復旧を宣言\n", recoveredAt.Format("2006-01-02 15:04:05"), h.repository.GetUserPreferredName(recoveredUser))

	channelURL := fmt.Sprintf("%sarchives/%s", h.workSpaceURL, channel.ID)

	// デフォルト値の設定
	title := "例: サービスAPIが応答停止"
	summary := "例: サービスAPIが応答しない"
	status := "解決済み"
	impact := "例: サービスが断続的にダウンし、最大で１割のユーザーが影響を受けました。"
	rootCause := "例: ExampleAPIのバグ、設定ミス"
	trigger := "例: 監視アラート、ユーザーからの報告"
	solution := "例: 切り戻し、データベースの再起動"
	actionItems := "例:\n- 【根本対応】原因となったエンドポイントの修正\n- 【緩和策】エラーハンドリングの追加"
	lessonsGood := "例: 迅速な対応により影響時間を最小限に抑えることができた"
	lessonsBad := "例: 初期対応時の情報共有が不十分だった"
	lessensLucky := "例: 障害発生がピーク時間外だったため影響が限定的だった"

	postmortemFileTitle := fmt.Sprintf("%s postmortem-%s", createdAt.Format("2006/01/02"), channel.Name)

	if h.aiRepository != nil {
		// タイトル生成
		t, err := h.aiRepository.GenerateTitle(incident.Description, formattedMessages)
		if err != nil {
			return fmt.Errorf("failed to GenerateTitle: %w", err)
		}
		title = t
		postmortemFileTitle = fmt.Sprintf("%s %s", createdAt.Format("2006/01/02"), t)

		// サマリー生成
		s, err := h.aiRepository.Summarize(incident.Description, formattedMessages)
		if err != nil {
			return fmt.Errorf("failed to Summarize: %w", err)
		}
		summary = s
		incident.Description = s

		// ステータス生成
		st, err := h.aiRepository.GenerateStatus(incident.Description, formattedMessages)
		if err != nil {
			return fmt.Errorf("failed to GenerateStatus: %w", err)
		}
		status = st

		// 影響分析生成
		i, err := h.aiRepository.GenerateImpact(incident.Description, formattedMessages)
		if err != nil {
			return fmt.Errorf("failed to GenerateImpact: %w", err)
		}
		impact = i

		// 根本原因生成
		rc, err := h.aiRepository.GenerateRootCause(incident.Description, formattedMessages)
		if err != nil {
			return fmt.Errorf("failed to GenerateRootCause: %w", err)
		}
		rootCause = rc

		// トリガー生成
		tr, err := h.aiRepository.GenerateTrigger(incident.Description, formattedMessages)
		if err != nil {
			return fmt.Errorf("failed to GenerateTrigger: %w", err)
		}
		trigger = tr

		// 解決策生成
		so, err := h.aiRepository.GenerateSolution(incident.Description, formattedMessages)
		if err != nil {
			return fmt.Errorf("failed to GenerateSolution: %w", err)
		}
		solution = so

		// アクションアイテム生成
		ai, err := h.aiRepository.GenerateActionItems(incident.Description, formattedMessages)
		if err != nil {
			return fmt.Errorf("failed to GenerateActionItems: %w", err)
		}
		actionItems = ai

		// 学んだ教訓生成
		lg, lb, ll, err := h.aiRepository.GenerateLessonsLearned(incident.Description, formattedMessages)
		if err != nil {
			return fmt.Errorf("failed to GenerateLessonsLearned: %w", err)
		}
		lessonsGood = lg
		lessonsBad = lb
		lessensLucky = ll

		// タイムライン整形
		formattedMessages, err = h.aiRepository.FormatTimeline(formattedMessages)
		if err != nil {
			return fmt.Errorf("failed to FormatTimeline: %w", err)
		}
	}

	rendered := postmortem.Render(title, createdAt.Format("2006-01-02 15:04:05"), author, status, summary, impact, rootCause, trigger, solution, actionItems, lessonsGood, lessonsBad, lessensLucky, formattedMessages, channelURL)

	if h.postmortemExporter != nil {
		service, err := h.repository.ServiceByID(h.ctx, incident.ServiceID)
		if err != nil {
			slog.Error("failed to ServiceByID", slog.Any("err", err), slog.Any("serviceID", incident.ServiceID))
		}

		url, err := h.postmortemExporter.ExportPostMortem(h.ctx, postmortemFileTitle, rendered, service)
		if err != nil {
			return fmt.Errorf("failed to ExportPostMortem: %w", err)
		}
		incident.PostMortemURL = url
	} else {
		// アップロードする
		url, err := h.repository.UploadFile(h.workSpaceURL, user.ID, channel.ID, postmortemFileTitle, title, rendered)
		if err != nil {
			return fmt.Errorf("failed to UploadFile: %w", err)
		}
		incident.PostMortemURL = url
	}

	_, _, err = h.repository.PostMessage(
		channel.ID,
		slack.MsgOptionText(fmt.Sprintf("✅️ポストモーテムを作成しました: %s", incident.PostMortemURL), false),
	)
	if err != nil {
		slog.Error("Failed to post postmortem created message", slog.Any("err", err))
	}

	if err := h.repository.SaveIncident(h.ctx, incident); err != nil {
		return fmt.Errorf("failed to SaveIncident: %w", err)
	}
	return nil
}

func parseSlackTimestamp(ts string) (time.Time, error) {
	parts := strings.Split(ts, ".")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid timestamp format: %s", ts)
	}

	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	fracPart := parts[1] + strings.Repeat("0", 9-len(parts[1])) // nanosecond補正
	nsec, err := strconv.ParseInt(fracPart, 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		loc = time.UTC
	}

	return time.Unix(sec, nsec).In(loc), nil
}

func (h *CallbackHandler) broadCastAnnouncement(channelID string, attachment slack.Attachment, service *entity.Service, addHere bool) error {
	// インシデントを取得して紐づけられたチャンネルを確認
	incident, err := h.repository.FindIncidentByChannel(h.ctx, channelID)
	if err != nil {
		slog.Error("failed to FindIncidentByChannel for linked channels", slog.Any("err", err))
	}

	// 投稿済みチャンネルを追跡して重複を防止
	postedChannels := make(map[string]bool)

	// 紐づけられたチャンネルに先に通知（重複防止のため）
	if incident != nil && len(incident.LinkedChannels) > 0 {
		for _, linked := range incident.LinkedChannels {
			// スレッド紐づけの場合はスレッドのみに投稿、チャンネル紐づけの場合は重複チェック
			if linked.ThreadTS != "" {
				// スレッド紐づけ：スレッドに投稿（reply_broadcastを使用）
				_, _, err := h.repository.PostMessage(
					linked.ChannelID,
					slack.MsgOptionAttachments(attachment),
					slack.MsgOptionTS(linked.ThreadTS),
					slack.MsgOptionBroadcast(),
				)
				if err != nil {
					return fmt.Errorf("failed to post to linked thread %s: %w", linked.ChannelID, err)
				}

				// reply_broadcastでチャンネル全体にも表示されるため、そのチャンネルは投稿済みとしてマーク
				postedChannels[linked.ChannelID] = true

				// 成功を通知
				linkedChannel, channelErr := h.repository.GetChannelByID(linked.ChannelID)
				if channelErr == nil {
					_, _, err = h.repository.PostMessage(
						channelID,
						slack.MsgOptionText(fmt.Sprintf("🔗 %s チャンネルの紐づけスレッドに通知しました", linkedChannel.Name), false),
					)
					if err != nil {
						slog.Error("Failed to post linked thread notification message", slog.Any("err", err))
					}
				}
			} else {
				// チャンネル紐づけ：重複チェックしてからチャンネルに投稿
				if postedChannels[linked.ChannelID] {
					continue
				}

				_, _, err := h.repository.PostMessage(
					linked.ChannelID,
					slack.MsgOptionAttachments(attachment),
				)
				if err != nil {
					return fmt.Errorf("failed to post to linked channel %s: %w", linked.ChannelID, err)
				}

				postedChannels[linked.ChannelID] = true

				// 成功を通知
				linkedChannel, channelErr := h.repository.GetChannelByID(linked.ChannelID)
				if channelErr == nil {
					_, _, err = h.repository.PostMessage(
						channelID,
						slack.MsgOptionText(fmt.Sprintf("🔗 %s チャンネルに通知しました", linkedChannel.Name), false),
					)
					if err != nil {
						slog.Error("Failed to post linked channel notification message", slog.Any("err", err))
					}
				}
			}
		}
	}

	// アナウンスチャンネルをマージして重複を除去（紐づけ処理後に処理）
	announceChannels := make(map[string]bool)

	// サービス固有のアナウンスチャンネルを追加
	if service != nil {
		for _, c := range service.AnnouncementChannels {
			announceChannels[c] = true
		}
	}

	// グローバルアナウンスチャンネルを追加
	if h.config != nil {
		for _, c := range h.config.GetGlobalAnnouncementChannels(h.ctx) {
			announceChannels[c] = true
		}
	}

	// アナウンスチャンネルに通知（重複チェック済み）
	for c := range announceChannels {
		cinfo, err := h.repository.GetChannelByName(c)
		if err != nil {
			slog.Error("failed to GetChannelByName", slog.Any("err", err), slog.Any("channel", c))
			continue
		}
		if cinfo == nil {
			continue
		}

		// 既に投稿済みでないかチェック
		if postedChannels[cinfo.ID] {
			continue
		}

		var msgOptions []slack.MsgOption
		// 設定ファイルのnotification_typeに基づいて通知方法を決定
		notificationType := "none"
		if h.config != nil {
			notificationType = h.config.GetNotificationType()
		}

		// addHereがtrueの場合のみ、notification_typeに従って通知を追加
		if addHere {
			notificationText := blocks.AddNotification("", notificationType)
			if notificationText != "" {
				msgOptions = append(msgOptions, slack.MsgOptionText(notificationText, false))
			}
		}
		msgOptions = append(msgOptions, slack.MsgOptionAttachments(attachment))

		_, _, err = h.repository.PostMessage(cinfo.ID, msgOptions...)
		if err != nil {
			return fmt.Errorf("failed to post announcement to channel %s: %w", cinfo.Name, err)
		}

		postedChannels[cinfo.ID] = true

		_, _, err = h.repository.PostMessage(
			channelID,
			slack.MsgOptionText(fmt.Sprintf("📢 %s チャンネルに通知しました", cinfo.Name), false),
		)
		if err != nil {
			slog.Error("Failed to post notification message", slog.Any("err", err))
		}
	}

	return nil
}

// 事象内容編集用のモーダルを開く
func (h *CallbackHandler) openEditSummaryModal(triggerID, channelID string) error {
	titleText := slack.NewTextBlockObject("plain_text", "📝 事象内容の編集", false, false)
	submitText := slack.NewTextBlockObject("plain_text", "✅ 更新", false, false)
	closeText := slack.NewTextBlockObject("plain_text", "❌ キャンセル", false, false)

	// 現在のインシデント情報を取得
	incident, err := h.repository.FindIncidentByChannel(h.ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to FindIncidentByChannel: %w", err)
	}

	if incident == nil {
		return fmt.Errorf("incident is nil")
	}

	view := slack.ModalViewRequest{
		Type:            slack.ViewType("modal"),
		Title:           titleText,
		CallbackID:      "edit_summary_modal",
		Submit:          submitText,
		Close:           closeText,
		Blocks:          blocks.EditIncidentSummary(incident.Description),
		PrivateMetadata: channelID,
	}

	err = h.repository.OpenView(triggerID, view)
	if err != nil {
		return err
	}

	return nil
}

// 事象内容編集モーダルの送信処理
func (h *CallbackHandler) submitEditSummaryModal(callback *slack.InteractionCallback) error {
	channelID := callback.View.PrivateMetadata
	summaryText := callback.View.State.Values["edit_summary_block"]["summary_text"].Value
	userID := callback.User.ID

	// インシデント情報を取得
	incident, err := h.repository.FindIncidentByChannel(h.ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to FindIncidentByChannel: %w", err)
	}

	if incident == nil {
		return fmt.Errorf("incident is nil")
	}

	// 古い事象内容を保存
	oldSummary := incident.Description

	// 新しい事象内容を設定
	incident.Description = summaryText
	if err := h.repository.SaveIncident(h.ctx, incident); err != nil {
		return fmt.Errorf("failed to SaveIncident: %w", err)
	}

	// チャンネルのトピックも更新
	channel, err := h.repository.GetChannelByID(channelID)
	if err != nil {
		return fmt.Errorf("failed to GetChannelByID: %w", err)
	}

	service, err := h.repository.ServiceByID(h.ctx, incident.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to ServiceByID: %w", err)
	}

	urgencyText, ok := blocks.UrgencyMap[incident.Urgency]
	if !ok {
		return fmt.Errorf("invalid urgency: %s", incident.Urgency)
	}

	// トピックを更新（復旧済みの場合は【復旧】のプレフィックスを維持）
	topic := fmt.Sprintf("サービス名:%s 緊急度:%s 事象内容:%s", service.Name, urgencyText, summaryText)
	if strings.HasPrefix(channel.Topic.Value, "【復旧】") {
		topic = fmt.Sprintf("【復旧】%s", topic)
	}

	err = h.repository.SetTopicOfConversation(channelID, topic)
	if err != nil {
		return fmt.Errorf("failed to SetTopicOfConversation: %w", err)
	}

	// 変更を通知
	_, _, err = h.repository.PostMessage(
		channelID,
		slack.MsgOptionText(fmt.Sprintf("✅ <@%s>が事象内容を更新しました\n*変更前:* %s\n*変更後:* %s", userID, oldSummary, summaryText), false),
	)
	if err != nil {
		slog.Error("Failed to post summary update message", slog.Any("err", err))
	}

	// 周知チャンネルに通知
	color := "#f2c744"
	if !incident.RecoveredAt.IsZero() {
		color = "#36a64f"
	}
	attachment := slack.Attachment{
		Color:  color,
		Blocks: slack.Blocks{BlockSet: blocks.IncidentSummaryUpdated(oldSummary, summaryText, channelID, service, !incident.RecoveredAt.IsZero())},
	}

	if err := h.broadCastAnnouncement(channelID, attachment, service, false); err != nil {
		return fmt.Errorf("failed to broadCastAnnouncement: %w", err)
	}

	return nil
}

// インシデントを再開する
func (h *CallbackHandler) reopenIncident(userID, channelID string) error {
	incident, err := h.repository.FindIncidentByChannel(h.ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to FindIncidentByChannel: %w", err)
	}

	if incident == nil {
		return fmt.Errorf("incident is nil")
	}

	// 既に復旧していない場合はエラー
	if incident.RecoveredAt.IsZero() {
		_, _, err := h.repository.PostMessage(
			channelID,
			slack.MsgOptionText("⚠️ インシデントはまだ復旧していません。復旧していないインシデントは再開できません。", false),
		)
		if err != nil {
			slog.Error("Failed to post incident not recovered message", slog.Any("err", err))
		}
		return nil
	}

	// インシデントを再開状態に更新
	incident.ReopenedAt = timeNow()
	incident.ReopenedUserID = userID
	incident.RecoveredAt = time.Time{} // 復旧時刻をリセット
	incident.RecoveredUserID = ""      // 復旧者をリセット
	incident.DisableTimer = false      // タイマーを再開

	if err := h.repository.SaveIncident(h.ctx, incident); err != nil {
		return fmt.Errorf("failed to SaveIncident: %w", err)
	}

	service, err := h.repository.ServiceByID(h.ctx, incident.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to ServiceByID: %w", err)
	}

	channel, err := h.repository.GetChannelByID(channelID)
	if err != nil {
		return fmt.Errorf("failed to GetChannelByID: %w", err)
	}

	// トピックから【復旧】プレフィックスを削除
	topic := strings.TrimPrefix(channel.Topic.Value, "【復旧】")
	err = h.repository.SetTopicOfConversation(channel.ID, topic)
	if err != nil {
		return fmt.Errorf("failed to SetTopicOfConversation: %w", err)
	}

	// 通知タイプを取得
	notificationType := "here"
	if h.config != nil {
		notificationType = h.config.GetNotificationType()
	}

	// チャンネル内に再開通知
	attachment := slack.Attachment{
		Color:  "#ff0000",
		Blocks: slack.Blocks{BlockSet: blocks.IncidentReopened(userID, incident.HandlerUserID, notificationType)},
	}

	_, _, err = h.repository.PostMessage(
		channelID,
		slack.MsgOptionAttachments(attachment),
	)
	if err != nil {
		slog.Error("Failed to post incident reopened message", slog.Any("err", err))
	}

	// アナウンスチャンネルに通知
	incidentLevel, err := h.repository.IncidentLevelByLevel(h.ctx, incident.Level)
	if err != nil {
		return fmt.Errorf("failed to IncidentLevelByLevel: %w", err)
	}

	attachment = slack.Attachment{
		Color: "#ff0000",
		Blocks: slack.Blocks{BlockSet: blocks.IncidentReopenedAnnounce(
			incident.Description,
			incidentLevel.Description,
			channel.ID,
			service,
		)},
	}

	if err := h.broadCastAnnouncement(channelID, attachment, service, false); err != nil {
		return fmt.Errorf("failed to broadCastAnnouncement: %w", err)
	}

	return nil
}

// 進捗サマリを作成する
func (h *CallbackHandler) createProgressSummary(channel slack.Channel, user slack.User) error {
	incident, err := h.repository.FindIncidentByChannel(h.ctx, channel.ID)
	if err != nil {
		return fmt.Errorf("failed to FindIncidentByChannel: %w", err)
	}

	if incident == nil {
		_, _, err := h.repository.PostMessage(
			channel.ID,
			slack.MsgOptionText("❌ このチャンネルにはインシデントが見つかりません", false),
		)
		if err != nil {
			slog.Error("Failed to post incident not found message", slog.Any("err", err))
		}
		return nil
	}

	// チャンネルメッセージを収集
	messages, err := h.collectChannelMessages(channel.ID, incident)
	if err != nil {
		return fmt.Errorf("failed to collect channel messages: %w", err)
	}

	if len(messages) == 0 {
		_, _, err := h.repository.PostMessage(
			channel.ID,
			slack.MsgOptionText("❌ 分析できるメッセージがありません", false),
		)
		if err != nil {
			slog.Error("Failed to post no messages message", slog.Any("err", err))
		}
		return nil
	}

	// AIで進捗サマリを生成（高度な分割処理対応）
	summary, err := h.aiRepository.SummarizeProgressAdvanced(
		incident.Description,
		messages,
		incident.LastSummary,
	)
	if err != nil {
		// フォールバック: 従来の方式でピンメッセージのみ使用
		return h.createProgressSummaryFallback(channel, incident)
	}

	// サマリを表示
	_, _, err = h.repository.PostMessage(
		channel.ID,
		slack.MsgOptionBlocks(blocks.ProgressSummary(summary)...),
	)
	if err != nil {
		return fmt.Errorf("failed to post progress summary: %w", err)
	}

	// インシデントにサマリ情報を保存
	err = h.updateIncidentSummary(incident, summary, messages)
	if err != nil {
		slog.Error("Failed to update incident summary", slog.Any("err", err))
		// エラーでも続行（サマリ表示は成功したため）
	}

	return nil
}

// チャンネルメッセージを収集
func (h *CallbackHandler) collectChannelMessages(channelID string, incident *entity.Incident) ([]slack.Message, error) {
	// 前回処理済みのタイムスタンプ以降のメッセージを取得
	var messages []slack.Message
	var err error

	if incident.LastProcessedMessageTS != "" {
		// 増分更新: 前回以降のメッセージのみ
		messages, err = h.repository.GetChannelMessagesAfter(channelID, incident.LastProcessedMessageTS)
	} else {
		// 初回作成: インシデント開始以降の全メッセージ
		oldest := fmt.Sprintf("%.6f", float64(incident.StartedAt.Unix()))
		messages, err = h.repository.GetChannelHistory(channelID, oldest, "", 1000)

		// スレッドも収集
		for _, msg := range messages {
			if msg.ThreadTimestamp != "" && msg.ThreadTimestamp == msg.Timestamp {
				replies, replyErr := h.repository.GetThreadReplies(channelID, msg.Timestamp)
				if replyErr == nil {
					// 元メッセージ以外の返信を追加
					for _, reply := range replies {
						if reply.Timestamp != msg.Timestamp {
							messages = append(messages, reply)
						}
					}
				}
			}
		}
	}

	if err != nil {
		return nil, err
	}

	// メッセージを時系列でソート
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp < messages[j].Timestamp
	})

	return messages, nil
}

// インシデントのサマリ情報を更新
func (h *CallbackHandler) updateIncidentSummary(incident *entity.Incident, summary string, messages []slack.Message) error {
	now := time.Now()
	incident.LastSummary = summary
	incident.LastSummaryAt = now

	// 最後に処理したメッセージのタイムスタンプを更新
	if len(messages) > 0 {
		incident.LastProcessedMessageTS = messages[len(messages)-1].Timestamp
	}

	return h.repository.SaveIncident(h.ctx, incident)
}

// フォールバック用のサマリ作成（従来の方式）
func (h *CallbackHandler) createProgressSummaryFallback(channel slack.Channel, incident *entity.Incident) error {
	slog.Warn("Using fallback progress summary method", slog.String("channelID", channel.ID))

	// ピンメッセージを取得
	pinnedMessages, err := h.repository.GetPinnedMessages(channel.ID)
	if err != nil {
		return fmt.Errorf("failed to GetPinnedMessages: %w", err)
	}

	// Slackメッセージを整形してタイムラインとして渡す
	var timeline strings.Builder
	for _, msg := range pinnedMessages {
		timeline.WriteString(fmt.Sprintf("%s: %s\n", msg.User, msg.Text))
	}

	// AIで進捗サマリを生成（従来の方式）
	summary, err := h.aiRepository.SummarizeProgress(incident.Description, timeline.String())
	if err != nil {
		_, _, postErr := h.repository.PostMessage(
			channel.ID,
			slack.MsgOptionText("❌ 進捗サマリの生成に失敗しました", false),
		)
		if postErr != nil {
			slog.Error("Failed to post summary generation error message", slog.Any("err", postErr))
		}
		return fmt.Errorf("failed to SummarizeProgress: %w", err)
	}

	// サマリを表示
	_, _, err = h.repository.PostMessage(
		channel.ID,
		slack.MsgOptionBlocks(blocks.ProgressSummary(summary)...),
	)
	if err != nil {
		return fmt.Errorf("failed to post progress summary: %w", err)
	}

	return nil
}

// 報告チャンネルに投稿する
func (h *CallbackHandler) postToReportChannel(channel slack.Channel, user slack.User, message slack.Message) error {
	incident, err := h.repository.FindIncidentByChannel(h.ctx, channel.ID)
	if err != nil {
		return fmt.Errorf("failed to FindIncidentByChannel: %w", err)
	}

	if incident == nil {
		_, _, err := h.repository.PostMessage(
			channel.ID,
			slack.MsgOptionText("❌ このチャンネルにはインシデントが見つかりません", false),
		)
		if err != nil {
			slog.Error("Failed to post incident not found message", slog.Any("err", err))
		}
		return nil
	}

	service, err := h.repository.ServiceByID(h.ctx, incident.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to ServiceByID: %w", err)
	}

	// アナウンスチャンネルの有無を確認
	hasAnnouncementChannels := len(service.AnnouncementChannels) > 0
	if h.config != nil {
		hasAnnouncementChannels = hasAnnouncementChannels || len(h.config.GetGlobalAnnouncementChannels(h.ctx)) > 0
	}

	if !hasAnnouncementChannels {
		_, _, err := h.repository.PostMessage(
			channel.ID,
			slack.MsgOptionText("❌ アナウンスチャンネルが設定されていません", false),
		)
		if err != nil {
			slog.Error("Failed to post no announcement channels message", slog.Any("err", err))
		}
		return nil
	}

	// メッセージブロックからサマリ部分を抽出
	var summaryText string
	var summaryParts []string

	for _, block := range message.Blocks.BlockSet {
		if sectionBlock, ok := block.(*slack.SectionBlock); ok {
			if sectionBlock.Text != nil && sectionBlock.Text.Type == "mrkdwn" {
				text := sectionBlock.Text.Text
				// ヘッダーブロック、ボタンブロック、区切り線は除外
				if text != "" && !strings.Contains(text, "進捗サマリ") && !strings.Contains(text, "報告chに投稿") {
					summaryParts = append(summaryParts, text)
				}
			}
		}
	}

	// 全てのセクションを結合してサマリテキストを作成
	if len(summaryParts) > 0 {
		summaryText = strings.Join(summaryParts, "\n\n")
	}

	if summaryText == "" {
		_, _, err := h.repository.PostMessage(
			channel.ID,
			slack.MsgOptionText("❌ 投稿するサマリが見つかりません", false),
		)
		if err != nil {
			slog.Error("Failed to post no summary found message", slog.Any("err", err))
		}
		return nil
	}

	// 既存のbroadCastAnnouncement機能を使用してアナウンスチャンネルに投稿
	attachment := slack.Attachment{
		Color:  "#36a64f",
		Blocks: slack.Blocks{BlockSet: blocks.ProgressSummaryAnnouncement(summaryText, channel.ID, service)},
	}

	if err := h.broadCastAnnouncement(channel.ID, attachment, service, false); err != nil {
		_, _, postErr := h.repository.PostMessage(
			channel.ID,
			slack.MsgOptionText("❌ アナウンスチャンネルへの投稿に失敗しました", false),
		)
		if postErr != nil {
			slog.Error("Failed to post broadcast error message", slog.Any("err", postErr))
		}
		return fmt.Errorf("failed to broadcast progress summary: %w", err)
	}

	// 投稿成功を通知
	_, _, err = h.repository.PostMessage(
		channel.ID,
		slack.MsgOptionBlocks(blocks.ReportPostSuccess("アナウンスチャンネル")...),
	)
	if err != nil {
		slog.Error("Failed to post success message", slog.Any("err", err))
	}

	return nil
}

// 進捗サマリを作成し、指定されたメッセージを更新する
func (h *CallbackHandler) createProgressSummaryWithUpdate(channel slack.Channel, user slack.User, updateMsgTS string) error {
	incident, err := h.repository.FindIncidentByChannel(h.ctx, channel.ID)
	if err != nil {
		// エラーメッセージで作成中メッセージを更新
		h.repository.UpdateMessage(
			channel.ID,
			updateMsgTS,
			slack.MsgOptionText("❌ インシデント情報の取得に失敗しました", false),
		)
		return fmt.Errorf("failed to FindIncidentByChannel: %w", err)
	}

	if incident == nil {
		h.repository.UpdateMessage(
			channel.ID,
			updateMsgTS,
			slack.MsgOptionText("❌ このチャンネルにはインシデントが見つかりません", false),
		)
		return nil
	}

	// チャンネルメッセージを収集
	messages, err := h.collectChannelMessages(channel.ID, incident)
	if err != nil {
		h.repository.UpdateMessage(
			channel.ID,
			updateMsgTS,
			slack.MsgOptionText("❌ メッセージの収集に失敗しました", false),
		)
		return fmt.Errorf("failed to collect channel messages: %w", err)
	}

	if len(messages) == 0 {
		h.repository.UpdateMessage(
			channel.ID,
			updateMsgTS,
			slack.MsgOptionText("❌ 分析できるメッセージがありません", false),
		)
		return nil
	}

	// AIで進捗サマリを生成（高度な分割処理対応）
	summary, err := h.aiRepository.SummarizeProgressAdvanced(
		incident.Description,
		messages,
		incident.LastSummary,
	)
	if err != nil {
		// フォールバック: 従来の方式でピンメッセージのみ使用
		return h.createProgressSummaryFallbackWithUpdate(channel, incident, updateMsgTS)
	}

	// サマリを表示（作成中メッセージを更新）
	h.repository.UpdateMessage(
		channel.ID,
		updateMsgTS,
		slack.MsgOptionBlocks(blocks.ProgressSummary(summary)...),
	)

	// インシデントにサマリ情報を保存
	err = h.updateIncidentSummary(incident, summary, messages)
	if err != nil {
		slog.Error("Failed to update incident summary", slog.Any("err", err))
		// エラーでも続行（サマリ表示は成功したため）
	}

	return nil
}

// フォールバック用のサマリ作成（作成中メッセージ更新版）
func (h *CallbackHandler) createProgressSummaryFallbackWithUpdate(channel slack.Channel, incident *entity.Incident, updateMsgTS string) error {
	slog.Warn("Using fallback progress summary method", slog.String("channelID", channel.ID))

	// ピンメッセージを取得
	pinnedMessages, err := h.repository.GetPinnedMessages(channel.ID)
	if err != nil {
		h.repository.UpdateMessage(
			channel.ID,
			updateMsgTS,
			slack.MsgOptionText("❌ ピンメッセージの取得に失敗しました", false),
		)
		return fmt.Errorf("failed to GetPinnedMessages: %w", err)
	}

	// Slackメッセージを整形してタイムラインとして渡す
	var timeline strings.Builder
	for _, msg := range pinnedMessages {
		timeline.WriteString(fmt.Sprintf("%s: %s\n", msg.User, msg.Text))
	}

	// AIで進捗サマリを生成（従来の方式）
	summary, err := h.aiRepository.SummarizeProgress(incident.Description, timeline.String())
	if err != nil {
		h.repository.UpdateMessage(
			channel.ID,
			updateMsgTS,
			slack.MsgOptionText("❌ 進捗サマリの生成に失敗しました", false),
		)
		return fmt.Errorf("failed to SummarizeProgress: %w", err)
	}

	// サマリを表示（作成中メッセージを更新）
	h.repository.UpdateMessage(
		channel.ID,
		updateMsgTS,
		slack.MsgOptionBlocks(blocks.ProgressSummary(summary)...),
	)

	return nil
}

// アクティブなインシデント一覧を表示するモーダルを開く
func (h *CallbackHandler) showActiveIncidentsList(callback *slack.InteractionCallback) error {
	// アクティブなインシデント（復旧していない）を取得
	incidents, err := h.repository.ActiveIncidents(h.ctx)
	if err != nil {
		return fmt.Errorf("failed to ActiveIncidents: %w", err)
	}

	if len(incidents) == 0 {
		_, _, err := h.repository.PostMessage(
			callback.Channel.ID,
			slack.MsgOptionText("現在アクティブなインシデントはありません", false),
		)
		if err != nil {
			slog.Error("Failed to post no active incidents message", slog.Any("err", err))
		}
		return nil
	}

	// インシデントを新しい順にソート（未解決のものを優先）
	sort.Slice(incidents, func(i, j int) bool {
		return incidents[i].StartedAt.After(incidents[j].StartedAt)
	})

	// インシデント選択用のオプションを作成
	var options []*slack.OptionBlockObject
	for _, incident := range incidents {
		service, err := h.repository.ServiceByID(h.ctx, incident.ServiceID)
		if err != nil {
			continue
		}

		channel, err := h.repository.GetChannelByID(incident.ChannelID)
		if err != nil {
			continue
		}

		// アーカイブ済みのチャンネルは除外
		if channel.IsArchived {
			continue
		}

		// インシデントの概要を作成
		description := fmt.Sprintf("%s - %s", service.Name, incident.Description)
		if len(description) > 75 {
			description = description[:72] + "..."
		}

		options = append(options, slack.NewOptionBlockObject(
			incident.ChannelID, // valueとしてチャンネルIDを使用
			slack.NewTextBlockObject("plain_text", fmt.Sprintf("#%s", channel.Name), false, false),
			slack.NewTextBlockObject("plain_text", description, false, false),
		))
	}

	// 紐づけ可能なインシデントがない場合
	if len(options) == 0 {
		_, _, err := h.repository.PostMessage(
			callback.Channel.ID,
			slack.MsgOptionText("紐づけ可能なアクティブなインシデントはありません", false),
		)
		if err != nil {
			slog.Error("Failed to post no linkable incidents message", slog.Any("err", err))
		}
		return nil
	}

	// モーダルを作成
	titleText := slack.NewTextBlockObject("plain_text", "🔗 インシデント選択", false, false)
	submitText := slack.NewTextBlockObject("plain_text", "✅ 紐づける", false, false)
	closeText := slack.NewTextBlockObject("plain_text", "❌ キャンセル", false, false)

	// コールバック情報をprivate_metadataに保存（スレッドタイムスタンプを含む）
	var threadTS string
	if callback.Message.ThreadTimestamp != "" {
		// スレッド内のメッセージの場合
		threadTS = callback.Message.ThreadTimestamp
	}
	// チャンネル直接の場合はthreadTSは空文字列のまま
	metadata := fmt.Sprintf("%s|%s", callback.Channel.ID, threadTS)

	view := slack.ModalViewRequest{
		Type:       slack.ViewType("modal"),
		Title:      titleText,
		CallbackID: "link_incident_modal",
		Submit:     submitText,
		Close:      closeText,
		Blocks: slack.Blocks{
			BlockSet: []slack.Block{
				slack.NewSectionBlock(
					slack.NewTextBlockObject(
						"mrkdwn",
						"紐づけたいインシデントを選択してください:",
						false,
						false,
					),
					nil,
					nil,
				),
				slack.NewActionBlock(
					"incident_select_action",
					slack.NewOptionsSelectBlockElement(
						slack.OptTypeStatic,
						slack.NewTextBlockObject("plain_text", "インシデントを選択", false, false),
						"incident_select",
						options...,
					),
				),
			},
		},
		PrivateMetadata: metadata,
	}

	err = h.repository.OpenView(callback.TriggerID, view)
	if err != nil {
		return fmt.Errorf("failed to OpenView: %w", err)
	}

	return nil
}

// インシデント紐づけモーダルの送信処理
func (h *CallbackHandler) submitLinkIncidentModal(callback *slack.InteractionCallback) error {
	// private_metadataから元のチャンネル情報を取得
	metadata := callback.View.PrivateMetadata
	parts := strings.Split(metadata, "|")
	if len(parts) != 2 {
		return fmt.Errorf("invalid private metadata format")
	}

	linkChannelID := parts[0]
	linkThreadTS := parts[1] // メッセージまたはスレッドのタイムスタンプ

	// スレッドかチャンネルかを判定し、適切なThreadTSを設定
	// linkThreadTSが空文字列でない場合のみスレッド紐づけとして扱う
	actualThreadTS := linkThreadTS

	// 選択されたインシデントチャンネルIDを取得
	incidentChannelID := callback.View.State.Values["incident_select_action"]["incident_select"].SelectedOption.Value

	// インシデントを取得
	incident, err := h.repository.FindIncidentByChannel(h.ctx, incidentChannelID)
	if err != nil {
		return fmt.Errorf("failed to FindIncidentByChannel: %w", err)
	}
	if incident == nil {
		return fmt.Errorf("incident not found")
	}

	// 既に紐づけられていないかチェック
	for _, linked := range incident.LinkedChannels {
		if linked.ChannelID == linkChannelID && linked.ThreadTS == actualThreadTS {
			var msgText string
			if actualThreadTS != "" {
				msgText = "このスレッドは既にインシデントに紐づけられています"
			} else {
				msgText = "このチャンネルは既にインシデントに紐づけられています"
			}

			msgOptions := []slack.MsgOption{
				slack.MsgOptionText(msgText, false),
			}
			if actualThreadTS != "" {
				msgOptions = append(msgOptions, slack.MsgOptionTS(actualThreadTS))
			}

			_, _, err := h.repository.PostMessage(linkChannelID, msgOptions...)
			if err != nil {
				return fmt.Errorf("failed to post already linked message: %w", err)
			}
			return nil
		}
	}

	// 新しい紐づけを追加
	if incident.LinkedChannels == nil {
		incident.LinkedChannels = []entity.LinkedChannel{}
	}

	incident.LinkedChannels = append(incident.LinkedChannels, entity.LinkedChannel{
		ChannelID: linkChannelID,
		ThreadTS:  actualThreadTS,
	})

	// インシデントを保存
	if err := h.repository.SaveIncident(h.ctx, incident); err != nil {
		return fmt.Errorf("failed to SaveIncident: %w", err)
	}

	// 紐づけ成功メッセージを投稿
	incidentChannel, err := h.repository.GetChannelByID(incidentChannelID)
	if err != nil {
		return fmt.Errorf("failed to GetChannelByID: %w", err)
	}

	// 紐づけ成功メッセージを送信
	var successMsgText string
	if actualThreadTS != "" {
		successMsgText = fmt.Sprintf("✅ このスレッドをインシデント <#%s> に紐づけました。今後のインシデント更新がここにも通知されます。", incidentChannelID)
	} else {
		successMsgText = fmt.Sprintf("✅ このチャンネルをインシデント <#%s> に紐づけました。今後のインシデント更新がここにも通知されます。", incidentChannelID)
	}

	msgOptions := []slack.MsgOption{
		slack.MsgOptionText(successMsgText, false),
	}
	if actualThreadTS != "" {
		msgOptions = append(msgOptions, slack.MsgOptionTS(actualThreadTS))
	}

	_, _, err = h.repository.PostMessage(linkChannelID, msgOptions...)
	if err != nil {
		return fmt.Errorf("failed to post link success message: %w", err)
	}

	// インシデントチャンネルにも通知
	linkChannel, err := h.repository.GetChannelByID(linkChannelID)
	if err != nil {
		return fmt.Errorf("failed to GetChannelByID: %w", err)
	}

	// インシデントチャンネルへの通知メッセージ
	var notifyMsgText string
	if actualThreadTS != "" {
		notifyMsgText = fmt.Sprintf("🔗 <#%s> のスレッドがこのインシデントに紐づけられました", linkChannelID)
	} else {
		notifyMsgText = fmt.Sprintf("🔗 <#%s> がこのインシデントに紐づけられました", linkChannelID)
	}

	_, _, err = h.repository.PostMessage(
		incidentChannelID,
		slack.MsgOptionText(notifyMsgText, false),
	)
	if err != nil {
		return fmt.Errorf("failed to post link notification to incident channel: %w", err)
	}

	slog.Info("Successfully linked channel to incident",
		slog.Any("incidentChannel", incidentChannel.Name),
		slog.Any("linkedChannel", linkChannel.Name),
		slog.Any("threadTS", actualThreadTS))

	return nil
}

// インシデントから紐づけを解除する
func (h *CallbackHandler) unlinkFromIncident(callback *slack.InteractionCallback) error {
	channelID := callback.Channel.ID
	var threadTS string
	if callback.Message.ThreadTimestamp != "" {
		threadTS = callback.Message.ThreadTimestamp
	}

	// 全てのアクティブなインシデントから該当の紐づけを検索・削除
	incidents, err := h.repository.ActiveIncidents(h.ctx)
	if err != nil {
		return fmt.Errorf("failed to ActiveIncidents: %w", err)
	}

	var foundIncident *entity.Incident
	var linkIndex = -1

	for i, incident := range incidents {
		for j, linked := range incident.LinkedChannels {
			if linked.ChannelID == channelID && linked.ThreadTS == threadTS {
				foundIncident = &incidents[i]
				linkIndex = j
				break
			}
		}
		if foundIncident != nil {
			break
		}
	}

	if foundIncident == nil {
		var msgText string
		if threadTS != "" {
			msgText = "このスレッドはインシデントに紐づけられていません"
		} else {
			msgText = "このチャンネルはインシデントに紐づけられていません"
		}

		msgOptions := []slack.MsgOption{
			slack.MsgOptionText(msgText, false),
		}
		if threadTS != "" {
			msgOptions = append(msgOptions, slack.MsgOptionTS(threadTS))
		}

		_, _, err := h.repository.PostMessage(channelID, msgOptions...)
		if err != nil {
			return fmt.Errorf("failed to post not linked message: %w", err)
		}
		return nil
	}

	// 紐づけを削除
	foundIncident.LinkedChannels = append(
		foundIncident.LinkedChannels[:linkIndex],
		foundIncident.LinkedChannels[linkIndex+1:]...,
	)

	// インシデントを保存
	if err := h.repository.SaveIncident(h.ctx, foundIncident); err != nil {
		return fmt.Errorf("failed to SaveIncident: %w", err)
	}

	// 解除成功メッセージを送信
	incidentChannel, err := h.repository.GetChannelByID(foundIncident.ChannelID)
	if err != nil {
		return fmt.Errorf("failed to GetChannelByID: %w", err)
	}

	var successMsgText string
	if threadTS != "" {
		successMsgText = fmt.Sprintf("✅ このスレッドとインシデント <#%s> の紐づけを解除しました", foundIncident.ChannelID)
	} else {
		successMsgText = fmt.Sprintf("✅ このチャンネルとインシデント <#%s> の紐づけを解除しました", foundIncident.ChannelID)
	}

	// 解除成功メッセージのみを表示（メニューは再表示しない）
	msgOptions := []slack.MsgOption{
		slack.MsgOptionText(successMsgText, false),
	}
	if threadTS != "" {
		msgOptions = append(msgOptions, slack.MsgOptionTS(threadTS))
	}

	_, _, err = h.repository.PostMessage(channelID, msgOptions...)
	if err != nil {
		slog.Error("Failed to post unlink success message", slog.Any("err", err))
	}

	// インシデントチャンネルにも通知
	var notifyMsgText string
	if threadTS != "" {
		notifyMsgText = fmt.Sprintf("🔓 <#%s> のスレッドの紐づけが解除されました", channelID)
	} else {
		notifyMsgText = fmt.Sprintf("🔓 <#%s> の紐づけが解除されました", channelID)
	}

	_, _, err = h.repository.PostMessage(
		foundIncident.ChannelID,
		slack.MsgOptionText(notifyMsgText, false),
	)
	if err != nil {
		slog.Error("Failed to post unlink notification to incident channel", slog.Any("err", err))
	}

	slog.Info("Successfully unlinked channel from incident",
		slog.Any("incidentChannel", incidentChannel.Name),
		slog.Any("unlinkedChannel", channelID),
		slog.Any("threadTS", threadTS))

	return nil
}

// 報告ボタンをメッセージから削除する
func (h *CallbackHandler) removeReportButtonFromMessage(channelID string, message slack.Message) error {
	var newBlocks []slack.Block

	for _, block := range message.Blocks.BlockSet {
		// 報告ボタンを含むアクションブロックをスキップ
		if actionBlock, ok := block.(*slack.ActionBlock); ok {
			hasReportButton := false
			for _, element := range actionBlock.Elements.ElementSet {
				if buttonElement, ok := element.(*slack.ButtonBlockElement); ok {
					if buttonElement.ActionID == "report_post_action" {
						hasReportButton = true
						break
					}
				}
			}
			// 報告ボタンを含むアクションブロックは除外
			if hasReportButton {
				continue
			}
		}
		newBlocks = append(newBlocks, block)
	}

	// メッセージを更新
	h.repository.UpdateMessage(
		channelID,
		message.Timestamp,
		slack.MsgOptionBlocks(newBlocks...),
	)
	return nil
}

// 未クローズインシデント一覧を表示する
func (h *CallbackHandler) listOpenIncidents(channelID, threadTS string) error {
	slog.Info("listOpenIncidents called", slog.String("channelID", channelID), slog.String("threadTS", threadTS))

	incidents, err := h.repository.ActiveIncidents(h.ctx)
	if err != nil {
		slog.Error("failed to ActiveIncidents", slog.Any("err", err))
		return fmt.Errorf("failed to ActiveIncidents: %w", err)
	}

	slog.Info("ActiveIncidents retrieved", slog.Int("count", len(incidents)))

	if len(incidents) == 0 {
		slog.Info("No open incidents, posting message")
		msgOptions := []slack.MsgOption{
			slack.MsgOptionText("現在、未クローズのインシデントはありません。", false),
		}
		if threadTS != "" {
			msgOptions = append(msgOptions, slack.MsgOptionTS(threadTS))
		}

		_, _, err := h.repository.PostMessage(channelID, msgOptions...)
		if err != nil {
			slog.Error("failed to post no incidents message", slog.Any("err", err))
			return fmt.Errorf("failed to post no incidents message: %w", err)
		}
		slog.Info("No incidents message posted successfully")
		return nil
	}

	// インシデントを新しい順にソート
	sort.Slice(incidents, func(i, j int) bool {
		return incidents[i].StartedAt.After(incidents[j].StartedAt)
	})

	// 一覧表示の開始メッセージを投稿
	headerMsg := fmt.Sprintf("📋 未クローズのインシデント一覧 (全%d件)", len(incidents))
	slog.Info("Posting header message", slog.String("headerMsg", headerMsg))
	msgOptions := []slack.MsgOption{
		slack.MsgOptionText(headerMsg, false),
	}
	if threadTS != "" {
		msgOptions = append(msgOptions, slack.MsgOptionTS(threadTS))
	}

	_, headerTS, err := h.repository.PostMessage(channelID, msgOptions...)
	if err != nil {
		slog.Error("failed to post header message", slog.Any("err", err))
		return fmt.Errorf("failed to post header message: %w", err)
	}
	slog.Info("Header message posted successfully", slog.String("headerTS", headerTS))

	// スレッド内で各インシデントを一件ずつ投稿
	for i, incident := range incidents {
		slog.Info("Posting incident detail", slog.Int("index", i+1), slog.String("channelID", incident.ChannelID))
		if err := h.postIncidentDetail(channelID, headerTS, &incident, i+1); err != nil {
			slog.Error("Failed to post incident detail", slog.Any("err", err), slog.Any("incident", incident.ChannelID))
			continue
		}
	}

	slog.Info("All incidents posted successfully")
	return nil
}

// 個別のインシデント詳細を投稿する
func (h *CallbackHandler) postIncidentDetail(channelID, threadTS string, incident *entity.Incident, index int) error {
	service, err := h.repository.ServiceByID(h.ctx, incident.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to ServiceByID: %w", err)
	}

	channel, err := h.repository.GetChannelByID(incident.ChannelID)
	if err != nil {
		return fmt.Errorf("failed to GetChannelByID: %w", err)
	}

	// アーカイブ済みのチャンネルはスキップ
	if channel.IsArchived {
		return nil
	}

	// インシデントレベルを取得
	var levelDescription string
	if incident.Level > 0 {
		incidentLevel, err := h.repository.IncidentLevelByLevel(h.ctx, incident.Level)
		if err == nil && incidentLevel != nil {
			levelDescription = incidentLevel.Description
		}
	}
	if levelDescription == "" {
		levelDescription = "サービスに影響なし"
	}

	// 残件を分析（AIが利用可能な場合のみ）
	var remainingTasks string
	if h.aiRepository != nil {
		// インシデント開始以降の全メッセージ（スレッド含む）を収集
		messages, err := h.collectChannelMessages(incident.ChannelID, incident)
		if err != nil {
			slog.Error("Failed to collect channel messages", slog.Any("err", err))
			remainingTasks = "チャンネルメッセージの取得に失敗しました"
		} else if len(messages) == 0 {
			remainingTasks = "チャンネルにメッセージがありません"
		} else {
			slog.Info("Collected messages for analysis", slog.Int("count", len(messages)))

			// メッセージを整形
			var formattedMessages strings.Builder
			for _, msg := range messages {
				formattedMessages.WriteString(fmt.Sprintf("%s: %s\n", msg.User, msg.Text))
			}

			// AI で残件分析
			tasks, err := h.aiRepository.AnalyzeRemainingTasks(incident.Description, formattedMessages.String())
			if err != nil {
				slog.Error("Failed to analyze remaining tasks", slog.Any("err", err))
				remainingTasks = "残件の分析に失敗しました"
			} else {
				remainingTasks = tasks
			}
		}
	} else {
		remainingTasks = "AI機能が無効のため分析できません"
	}

	// 復旧状態の確認
	var statusEmoji string
	var statusText string
	if !incident.RecoveredAt.IsZero() {
		statusEmoji = "✅"
		statusText = "復旧済み（クローズ待ち）"
	} else {
		statusEmoji = "🚨"
		statusText = "対応中"
	}

	// メッセージブロックを作成
	blocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				fmt.Sprintf("*%d. %s %s*", index, statusEmoji, channel.Name),
				false,
				false,
			),
			nil,
			nil,
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				fmt.Sprintf("*サービス:* %s\n*ステータス:* %s\n*レベル:* %s",
					service.Name,
					statusText,
					levelDescription,
				),
				false,
				false,
			),
			nil,
			nil,
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				fmt.Sprintf("*概要:*\n%s", incident.Description),
				false,
				false,
			),
			nil,
			nil,
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				fmt.Sprintf("*残件:*\n%s", remainingTasks),
				false,
				false,
			),
			nil,
			nil,
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				fmt.Sprintf("*チャンネル:* <#%s>", incident.ChannelID),
				false,
				false,
			),
			nil,
			nil,
		),
		slack.NewDividerBlock(),
	}

	// スレッドに投稿
	_, _, err = h.repository.PostMessage(
		channelID,
		slack.MsgOptionBlocks(blocks...),
		slack.MsgOptionTS(threadTS),
	)
	if err != nil {
		return fmt.Errorf("failed to post incident detail: %w", err)
	}

	return nil
}
