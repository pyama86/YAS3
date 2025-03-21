package blocks

import "github.com/slack-go/slack"

func AlreadyRecovered() []slack.Block {
	return []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				"✅️インシデントは既に復旧しています。",
				false,
				false,
			),
			nil,
			nil,
		),
	}
}
