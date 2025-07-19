package blocks

import (
	"github.com/slack-go/slack"
)

// LinkIncidentMenu は非インシデントチャンネル用のメニュー
func LinkIncidentMenu(isThread bool, isLinked bool) []slack.Block {
	blocks := []slack.Block{}

	// インシデント作成ボタンを最初に表示
	blocks = append(blocks,
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				"新しいインシデントを作成する場合:",
				false,
				false,
			),
			nil,
			slack.NewAccessory(
				slack.NewButtonBlockElement(
					"incident_action",
					"create_new_incident",
					slack.NewTextBlockObject("plain_text", "🚨 インシデント作成", false, false),
				).WithStyle(slack.StyleDanger),
			),
		),
		slack.NewDividerBlock(),
	)

	// 紐づけメニューを下に表示
	blocks = append(blocks,
		slack.NewActionBlock("link_incident_action",
			slack.NewOptionsSelectBlockElement(
				slack.OptTypeStatic,
				slack.NewTextBlockObject("plain_text", "操作を選択してください", false, false),
				"link_incident_options",
				LinkIncidentOptions(isThread, isLinked)...,
			),
			slack.NewButtonBlockElement(
				"cancel_action",
				"cancel_button",
				slack.NewTextBlockObject("plain_text", "❌ キャンセル", false, false),
			),
		),
	)

	return blocks
}
