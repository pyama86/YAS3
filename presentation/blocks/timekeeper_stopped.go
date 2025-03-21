package blocks

import "github.com/slack-go/slack"

func TimeKeeperStopped(userID string) []slack.Block {
	return []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				"⛔️<@"+userID+">さんがタイムキーパーを停止しました。",
				false,
				false,
			),
			nil,
			nil,
		),
	}
}
