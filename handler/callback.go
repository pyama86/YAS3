package handler

import (
	"context"
	"fmt"
	"log/slog"
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
	ctx             context.Context
	repository      repository.Repository
	slackRepository *repository.SlackRepository
	aiRepository    *repository.AIRepository
	workSpaceURL    string
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
	slackRepository *repository.SlackRepository,
	aiRepository *repository.AIRepository,
	workSpaceURL string,
) *CallbackHandler {
	return &CallbackHandler{
		ctx:             ctx,
		repository:      repository,
		slackRepository: slackRepository,
		aiRepository:    aiRepository,
		workSpaceURL:    workSpaceURL,
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
			if err := h.openIncidentModal(callback.TriggerID, callback.Channel.ID, callback.User.ID); err != nil {
				return fmt.Errorf("openIncidentModal failed: %w", err)
			}

			h.slackRepository.UpdateMessage(
				callback.Channel.ID,
				callback.Message.Timestamp,
				slack.MsgOptionBlocks(blocks.UserIsTyping(callback.User.ID)...),
			)
		case "handler_button":
			h.slackRepository.DeleteMessage(
				callback.Channel.ID,
				callback.Message.Timestamp,
			)
			if err := h.submitHandler(callback.User.ID, callback.Channel.ID); err != nil {
				return fmt.Errorf("submitHandler failed: %w", err)
			}
		case "incident_level_button":
			h.slackRepository.DeleteMessage(
				callback.Channel.ID,
				callback.Message.Timestamp,
			)

			slog.Info("incident_level_options", slog.Any("channelID", callback.Channel.ID), slog.Any("value", callback.ActionCallback.BlockActions[0].Value))

			if err := h.setIncidentLevel(callback.Channel.ID, callback.User.ID, callback.ActionCallback.BlockActions[0].Value); err != nil {
				return fmt.Errorf("setIncidentLevel failed: %w", err)
			}
		case "postmortem_action":
			h.slackRepository.UpdateMessage(
				callback.Channel.ID,
				callback.Message.Timestamp,
				slack.MsgOptionText("📝 ポストモーテムを作成中...", false),
			)
			if err := h.createPostMortem(callback.Channel, callback.User); err != nil {
				return fmt.Errorf("createPostMortem failed: %w", err)
			}
		case "in_channel_options":
			if action.BlockID == "keeper_action" {
				currentBlocks := callback.Message.Blocks.BlockSet
				if len(currentBlocks) > 0 {
					currentBlocks = currentBlocks[:len(currentBlocks)-1]
				}

				h.slackRepository.UpdateMessage(
					callback.Channel.ID,
					callback.Message.Timestamp,
					slack.MsgOptionBlocks(currentBlocks...),
				)

			} else {
				h.slackRepository.DeleteMessage(
					callback.Channel.ID,
					callback.Message.Timestamp,
				)
			}
			switch callback.ActionCallback.BlockActions[0].SelectedOption.Value {
			case "recovery_incident":
				slog.Info("recovery_incident", slog.Any("channelID", callback.Channel.ID))
				if err := h.recoveryIncident(callback.User.ID, callback.Channel.ID); err != nil {
					return fmt.Errorf("recoveryIncident failed: %w", err)
				}

			case "stop_timekeeper":
				slog.Info("stop_timekeeper", slog.Any("channelID", callback.Channel.ID))
				if err := h.stopTimeKeeper(callback.Channel.ID, callback.User.ID); err != nil {
					return fmt.Errorf("stopTimeKeeper failed: %w", err)
				}
			case "set_incident_level":
				slog.Info("set_incident_level", slog.Any("channelID", callback.Channel.ID))
				h.showIncidentLevelButtons(callback.Channel.ID)
			case "create_postmortem":
				slog.Info("create_postmortem", slog.Any("channelID", callback.Channel.ID))
				if err := h.showPostMortemButton(callback.Channel.ID); err != nil {
					return fmt.Errorf("showPostMortemButton failed: %w", err)
				}
			}

		}
	case slack.InteractionTypeViewSubmission:
		switch callback.View.CallbackID {
		case "incident_modal":
			if err := h.submitIncidentModal(callback); err != nil {
				return fmt.Errorf("submitIncidentModal failed: %w", err)
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

	h.slackRepository.PostMessage(
		channelID,
		slack.MsgOptionBlocks(blocks.AcceptIncidentHandler(userID)...),
	)

	return nil
}
func (h *CallbackHandler) openIncidentModal(triggerID, channelID, userID string) error {
	titleText := slack.NewTextBlockObject("plain_text", "🚨 インシデントチャンネル作成", false, false)
	submitText := slack.NewTextBlockObject("plain_text", "✅ 作成", false, false)
	closeText := slack.NewTextBlockObject("plain_text", "❌ キャンセル", false, false)

	services, err := h.repository.Services(h.ctx)
	if err != nil {
		return err
	}

	channelInfo, err := h.slackRepository.GetChannelByID(channelID)
	if err != nil {
		return err
	}

	channelName := fmt.Sprintf("%s-%s", channelInfo.Name, timeNow().Format("2006-01-02"))

	view := slack.ModalViewRequest{
		Type:            slack.ViewType("modal"),
		Title:           titleText,
		CallbackID:      "incident_modal",
		Submit:          submitText,
		Close:           closeText,
		Blocks:          blocks.CreateIncident(channelName, services),
		PrivateMetadata: channelID,
	}

	err = h.slackRepository.OpenView(triggerID, view)
	if err != nil {
		return err
	}

	return err
}

func (h *CallbackHandler) submitIncidentModal(callback *slack.InteractionCallback) error {
	serviceID := callback.View.State.Values["service_block"]["service_select"].SelectedOption.Value
	summaryText := callback.View.State.Values["incident_summary_block"]["summary_text"].Value
	channelName := callback.View.State.Values["channel_name_block"]["channel_name_text"].Value
	urgency := callback.View.State.Values["urgency_block"]["urgency_select"].SelectedOption.Value
	userID := callback.User.ID

	slog.Info("submitIncidentModal", slog.Any("serviceID", serviceID), slog.Any("summary_text", summaryText), slog.Any("channelName", channelName), slog.Any("urgency", urgency))

	// チャンネル作成
	c, err := h.slackRepository.GetChannelByName(channelName)
	if err != nil && err != repository.ErrSlackNotFound {
		return fmt.Errorf("failed to GetChannelByID: %w", err)
	}
	if c != nil {
		channelName = fmt.Sprintf("%s-%02d", channelName, timeNow().Unix()%100)
	}

	channel, err := h.slackRepository.CreateConversation(slack.CreateConversationParams{
		ChannelName: channelName,
	})
	if err != nil {
		h.slackRepository.PostMessage(
			callback.Channel.ID,
			slack.MsgOptionText(fmt.Sprintf("❌ チャンネルの作成に失敗しました:%s", err), false),
		)

		return fmt.Errorf("failed to CreateConversation: %w", err)
	}
	h.slackRepository.FlushChannelCache()
	num, err := strconv.Atoi(serviceID)
	if err != nil {
		return fmt.Errorf("failed to strconv.Atoi: %w", err)
	}

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
	if err := h.repository.SaveIncident(h.ctx, incident); err != nil {
		return fmt.Errorf("failed to SaveIncident: %w", err)
	}

	service, err := h.repository.ServiceByID(h.ctx, num)
	if err != nil {
		return fmt.Errorf("failed to ServiceByID: %w", err)
	}

	urgencyText, ok := blocks.UrgencyMap[urgency]
	if !ok {
		return fmt.Errorf("invalid urgency: %s", urgency)
	}

	topic := fmt.Sprintf("サービス名:%s 緊急度:%s 事象内容:%s", service.Name, urgencyText, summaryText)
	err = h.slackRepository.SetTopicOfConversation(channel.ID, topic)
	if err != nil {
		return fmt.Errorf("failed to SetPurposeOfConversation: %w", err)
	}
	var members []string
	errMembers := []string{}
	for _, member := range service.IncidentTeamMembers {
		memberIDs, err := h.slackRepository.GetMemberIDs(member)
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
		err = h.slackRepository.InviteUsersToConversation(channel.ID, members...)
		if err != nil {
			return fmt.Errorf("failed to InviteUsersToConversation: %w", err)
		}

		h.slackRepository.PostMessage(
			channel.ID,
			slack.MsgOptionBlocks(blocks.InviteMembers(service)...),
		)
	}

	if len(errMembers) > 0 {
		h.slackRepository.PostMessage(
			channel.ID,
			slack.MsgOptionText(fmt.Sprintf("❌ チームメンバーの取得に失敗しました:%s", strings.Join(errMembers, ",")), false),
		)
	}

	attachment := slack.Attachment{
		Color:  urgencyColorMap[urgency],
		Blocks: slack.Blocks{BlockSet: blocks.IncidentCreated(summaryText, urgencyText, channel.ID, service)},
	}
	h.slackRepository.PostMessage(
		channel.ID,
		slack.MsgOptionAttachments(attachment),
	)

	// 共有チャンネルにお知らせを投稿
	for _, c := range h.repository.AnnouncementChannels(h.ctx) {
		cinfo, err := h.slackRepository.GetChannelByName(c)
		if err != nil {
			slog.Error("failed to GetChannelByName", slog.Any("err", err), slog.Any("channel", c))
		}
		if cinfo == nil {
			continue
		}

		h.slackRepository.PostMessage(
			cinfo.ID,
			slack.MsgOptionAttachments(attachment),
		)
		h.slackRepository.PostMessage(
			channel.ID,
			slack.MsgOptionText(fmt.Sprintf("📢 %s チャンネルに通知しました", cinfo.Name), false),
		)
	}

	h.slackRepository.PostMessage(
		channel.ID,
		slack.MsgOptionBlocks(blocks.IncidentReportRequest(userID)...),
	)
	if err != nil {
		return fmt.Errorf("failed to PostMessage: %w", err)
	}

	h.slackRepository.PostMessage(
		channel.ID,
		slack.MsgOptionBlocks(blocks.HandlerRecruitmentMessage()...),
	)
	if err != nil {
		return fmt.Errorf("failed to PostMessage: %w", err)
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
		h.slackRepository.PostMessage(
			channelID,
			slack.MsgOptionBlocks(blocks.AlreadyRecovered()...),
		)
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

	channel, err := h.slackRepository.GetChannelByID(channelID)
	if err != nil {
		return fmt.Errorf("failed to GetChannelByID: %w", err)
	}

	topic := fmt.Sprintf("【復旧】%s", channel.Topic.Value)
	err = h.slackRepository.SetTopicOfConversation(channel.ID, topic)
	if err != nil {
		return fmt.Errorf("failed to SetPurposeOfConversation: %w", err)
	}
	attachment := slack.Attachment{
		Color:  "#36a64f",
		Blocks: slack.Blocks{BlockSet: blocks.IncidentRecovered(userID, incident.HandlerUserID)},
	}

	h.slackRepository.PostMessage(
		channelID,
		slack.MsgOptionAttachments(attachment),
	)

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

	for _, c := range h.repository.AnnouncementChannels(h.ctx) {
		cinfo, err := h.slackRepository.GetChannelByName(c)
		if err != nil {
			slog.Error("failed to GetChannelByName", slog.Any("err", err), slog.Any("channel", c))
		}
		if cinfo == nil {
			continue
		}
		h.slackRepository.PostMessage(
			cinfo.ID,
			slack.MsgOptionAttachments(attachment),
		)
		h.slackRepository.PostMessage(
			channelID,
			slack.MsgOptionText(fmt.Sprintf("📢 %s チャンネルに通知しました", cinfo.Name), false),
		)
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

	h.slackRepository.PostMessage(
		channelID,
		slack.MsgOptionBlocks(blocks.TimeKeeperStopped(userID)...),
	)
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
	if levelInt > 0 {
		color = "#f2c744"
	}
	qttachment := slack.Attachment{
		Color: color,
		Blocks: slack.Blocks{
			BlockSet: blocks.IncidentLevelUpdated(
				incident.Description,
				description,
				channelID,
				service,
			),
		},
	}

	h.slackRepository.PostMessage(
		channelID,
		slack.MsgOptionBlocks(blocks.IncidentLevelChanged(userID, description)...),
	)

	for _, c := range h.repository.AnnouncementChannels(h.ctx) {
		cinfo, err := h.slackRepository.GetChannelByName(c)
		if err != nil {
			slog.Error("failed to GetChannelByName", slog.Any("err", err), slog.Any("channel", c))
		}
		if cinfo == nil {
			continue
		}
		h.slackRepository.PostMessage(
			cinfo.ID,
			slack.MsgOptionAttachments(qttachment),
		)
		h.slackRepository.PostMessage(
			channelID,
			slack.MsgOptionText(fmt.Sprintf("📢 %s チャンネルに通知しました", cinfo.Name), false),
		)
	}
	return nil
}

// インシデントレベルの選択肢を提供する
func (h *CallbackHandler) showIncidentLevelButtons(channelID string) {
	levels := h.repository.IncidentLevels(h.ctx)
	h.slackRepository.PostMessage(
		channelID,
		slack.MsgOptionBlocks(blocks.IncidentLevelButtons(levels)...),
	)
}

// ピンを打つアナウンスをして、ボタンを表示する
func (h *CallbackHandler) showPostMortemButton(channelID string) error {
	incident, err := h.repository.FindIncidentByChannel(h.ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to FindIncidentByChannel: %w", err)
	}

	if incident.RecoveredAt.IsZero() {
		h.slackRepository.PostMessage(
			channelID,
			slack.MsgOptionText("⛔️まだインシデントが復旧していません", false),
		)
		return nil
	}

	h.slackRepository.PostMessage(
		channelID,
		slack.MsgOptionBlocks(blocks.PostMortemButton()...),
	)
	return nil
}

// AIを活用して、ポストモーテムを作成する
func (h *CallbackHandler) createPostMortem(channel slack.Channel, user slack.User) error {
	incident, err := h.repository.FindIncidentByChannel(h.ctx, channel.ID)
	if err != nil {
		return fmt.Errorf("failed to FindIncidentByChannel: %w", err)
	}

	createdAt := incident.StartedAt
	recoveredAt := incident.RecoveredAt

	author := h.slackRepository.GetUserPreferredName(&user)

	createdUser, err := h.slackRepository.GetUserByID(incident.CreatedUserID)
	if err != nil {
		return fmt.Errorf("failed to GetUserByID: %w", err)
	}

	recoveredUser, err := h.slackRepository.GetUserByID(incident.RecoveredUserID)
	if err != nil {
		return fmt.Errorf("failed to GetUserByID: %w", err)
	}

	pinnedMessages, err := h.slackRepository.GetPinnedMessages(channel.ID)
	if err != nil {
		return fmt.Errorf("failed to GetPinnedMessages: %w", err)
	}

	formattedMessages := fmt.Sprintf("- %s %sさんがインシデントチャンネルを作成しました\n", createdAt.Format("2006-01-02 15:04:05"), h.slackRepository.GetUserPreferredName(createdUser))
	// - yyyy-MM-dd HH:mm:ss messageの形式でメッセージを取得

	type message struct {
		ts   time.Time
		text string
	}

	var messages []message
	for _, m := range pinnedMessages {
		ts, err := parseSlackTimestamp(m.Timestamp)
		if err != nil {
			return fmt.Errorf("failed to parseSlackTimestamp: %w", err)
		}
		messages = append(messages, message{ts: ts, text: m.Text})
	}

	// messageを時系列順に並び替え
	for i := 0; i < len(messages); i++ {
		for j := i + 1; j < len(messages); j++ {
			if messages[i].ts.After(messages[j].ts) {
				tmp := messages[i]
				messages[i] = messages[j]
				messages[j] = tmp
			}
		}
	}

	for _, m := range messages {
		formattedMessages += fmt.Sprintf("- %s %s\n", m.ts.Format("2006-01-02 15:04:05"), m.text)
	}
	formattedMessages += fmt.Sprintf("- %s %sさんがインシデントを復旧を宣言\n", recoveredAt.Format("2006-01-02 15:04:05"), h.slackRepository.GetUserPreferredName(recoveredUser))

	channelURL := fmt.Sprintf("%sarchives/%s", h.workSpaceURL, channel.ID)
	title := "例: サービスAPIが応答停止"
	summary := "例: サービスAPIが応答しない"

	if h.aiRepository != nil {
		t, err := h.aiRepository.GenerateTitle(incident.Description, formattedMessages)
		if err != nil {
			return fmt.Errorf("failed to GenerateTitle: %w", err)
		}
		s, err := h.aiRepository.Summarize(incident.Description, formattedMessages)
		if err != nil {
			return fmt.Errorf("failed to Summarize: %w", err)
		}
		title = t
		summary = s
	}

	rendered := postmortem.Render(title, createdAt.Format("2006-01-02 15:04:05"), author, summary, formattedMessages, channelURL)
	if err != nil {
		return fmt.Errorf("failed to Render: %w", err)
	}

	// アップロードする
	filename := fmt.Sprintf("postmortem-%s.md", channel.Name)
	err = h.slackRepository.UploadFile(channel.ID, filename, title, rendered)
	if err != nil {
		return fmt.Errorf("failed to UploadFile: %w", err)
	}

	h.slackRepository.PostMessage(
		channel.ID,
		slack.MsgOptionText("✅️ポストモーテムを作成しました", false),
	)
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
