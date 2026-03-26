package blocks

import "github.com/slack-go/slack"

// ポストモーテムを作成するボタンを表示する
func PostMortemButton() []slack.Block {
	return []slack.Block{
		slack.NewHeaderBlock(
			slack.NewTextBlockObject("plain_text", "📝 ポストモーテムを作成しましょう！", false, false),
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", "🚀 この事象に関するポストモーテムを作成しましょう。\nチャンネルの履歴から自動生成します。作成ボタンを押下してください", false, false),
			nil,
			nil,
		),
		slack.NewDividerBlock(),
		slack.NewActionBlock(
			"postmortem_action",
			slack.NewButtonBlockElement(
				"postmortem_action",
				"postmortem_button",
				slack.NewTextBlockObject("plain_text", "📝 ポストモーテムを作成する", false, false),
			).WithStyle(slack.StylePrimary),
		),
	}
}
