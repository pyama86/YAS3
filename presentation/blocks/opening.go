package blocks

import "github.com/slack-go/slack"

func Opening() []slack.Block {
	return []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", "*呼んでいただき大感謝!!1*\n\n:wave: こんにちは、私はインシデント管理をサポートするボットです。", false, false),
			nil, nil,
		),

		slack.NewDividerBlock(),
		&slack.SectionBlock{
			Type:    slack.MBTSection,
			BlockID: "section-1",
			Text: &slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: "*インシデントですか？*",
			},
			Accessory: &slack.Accessory{
				ButtonElement: &slack.ButtonBlockElement{
					Type:     slack.METButton,
					ActionID: "incident_action",
					Value:    "dummy_value",
					Text: &slack.TextBlockObject{
						Type: "plain_text",
						Text: "チャンネルを作る!",
					},
					Style: slack.StyleDanger,
				},
			},
		},
	}
}
