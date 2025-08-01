package blocks

import (
	"fmt"

	"github.com/pyama86/YAS3/domain/entity"
	"github.com/slack-go/slack"
)

func IncidentSummaryUpdated(oldSummary, newSummary, channelID string, service *entity.Service, isRecovered bool) []slack.Block {
	titleText := "📝 事象内容が変更されました"
	if isRecovered {
		titleText = "✅【復旧済み】事象内容が変更されました"
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
					fmt.Sprintf("*変更前:* %s", oldSummary),
					false,
					false,
				),
				slack.NewTextBlockObject(
					"mrkdwn",
					fmt.Sprintf("*変更後:* %s", newSummary),
					false,
					false,
				),
				slack.NewTextBlockObject(
					"mrkdwn",
					fmt.Sprintf("*対応チャンネル:* %s", fmt.Sprintf("<#%s>", channelID)),
					false,
					false,
				),
			},
			nil,
		),
	}
}
