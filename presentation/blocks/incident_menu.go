package blocks

import (
	"github.com/slack-go/slack"
)

func IncidentMenu() []slack.Block {
	return []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				"ご要件は何でしょうか？",
				false,
				false,
			),
			nil,
			nil,
		),
		slack.NewActionBlock("in_channel_action", slack.NewOptionsSelectBlockElement(
			slack.OptTypeStatic,
			slack.NewTextBlockObject("plain_text", "操作を選択してください", false, false),
			"in_channel_options",
			InChannelOptions()...,
		)),
	}
}
