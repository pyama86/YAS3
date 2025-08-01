package blocks

import (
	"fmt"

	"github.com/pyama86/YAS3/domain/entity"
	"github.com/slack-go/slack"
)

func IncidentLevelUpdated(summaryText, levelText, channelName string, service *entity.Service, isRecovered bool) []slack.Block {
	titleText := "🚨 事象レベルが変更されました"
	if isRecovered {
		titleText = "✅【復旧済み】事象レベルが変更されました"
	}
	return []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", titleText, false, false),
			[]*slack.TextBlockObject{
				slack.NewTextBlockObject(
					"mrkdwn",
					fmt.Sprintf("*サービス名:* %s", service.Name),
					false,
					false,
				),
				slack.NewTextBlockObject(
					"mrkdwn",
					fmt.Sprintf("*事象レベル:* %s", levelText),
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
