package blocks

import (
	"fmt"

	"github.com/pyama86/YAS3/domain/entity"
	"github.com/slack-go/slack"
)

func IncidentRecovered(userID, handlerID, notificationType string) []slack.Block {
	notificationText := AddNotification("インシデントが復旧しました。", notificationType)

	return []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("<@%s> さんがインシデントの復旧を宣言しました", userID), false, false),
			nil,
			nil,
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", notificationText, false, false),
			nil,
			nil,
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("<@%s> さん、ハンドラーありがとうございました。残作業について最後に指揮をお願いします。", handlerID), false, false),
			nil,
			nil,
		),

		slack.NewRichTextBlock("残作業を共有してください",
			slack.NewRichTextList(slack.RTEListOrdered, 0,
				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement("今後の方針を示してください", nil),
				),
				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement("ポストモーテムの作成を促してください", nil),
				),
				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement("すべての作業が完了したらチャンネルをアーカイブしてください", nil),
				),
			),
		),
	}
}
func IncidentRecoverdAnnounce(summaryText, incidentLevel, channelName string, service *entity.Service) []slack.Block {
	return []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", "✅ インシデントが復旧しました", false, false),
			[]*slack.TextBlockObject{
				slack.NewTextBlockObject(
					"mrkdwn",
					fmt.Sprintf("*サービス名:* %s", service.Name),
					false,
					false,
				),
				slack.NewTextBlockObject(
					"mrkdwn",
					fmt.Sprintf("*事象レベル:* %s", incidentLevel),
					false,
					false,
				),
				slack.NewTextBlockObject(
					"mrkdwn",
					fmt.Sprintf("*事象内容:* %s", summaryText),
					false,
					false,
				),
				slack.NewTextBlockObject(
					"mrkdwn",
					fmt.Sprintf("*対応チャンネル:* %s", fmt.Sprintf("<#%s>", channelName)),
					false,
					false,
				),
			},
			nil,
		),
	}
}
