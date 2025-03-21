package blocks

import (
	"fmt"

	"github.com/pyama86/YAS3/domain/entity"
	"github.com/slack-go/slack"
)

func IncidentCreated(summaryText, urgencyText, channelName string, service *entity.Service) []slack.Block {
	return []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", "🚨 インシデントが発生しています", false, false),
			[]*slack.TextBlockObject{
				slack.NewTextBlockObject(
					"mrkdwn",
					fmt.Sprintf("*サービス名:* %s", service.Name),
					false,
					false,
				),
				slack.NewTextBlockObject(
					"mrkdwn",
					fmt.Sprintf("*緊急度:* %s", urgencyText),
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
