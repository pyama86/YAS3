package blocks

import "github.com/slack-go/slack"

func UserIsTyping(userID string) []slack.Block {
	return []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				"<@"+userID+"> さんが *インシデントの概要* を入力しています。",
				false,
				false,
			),
			nil,
			nil,
		),
	}
}
