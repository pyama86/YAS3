package blocks

import (
	"github.com/slack-go/slack"
)

func EditIncidentSummary(currentSummary string) slack.Blocks {
	return slack.Blocks{
		BlockSet: []slack.Block{
			// 事象内容
			&slack.InputBlock{
				Type:    slack.MBTInput,
				BlockID: "edit_summary_block",
				Label: &slack.TextBlockObject{
					Type: "plain_text",
					Text: "事象内容を編集",
				},
				Element: &slack.PlainTextInputBlockElement{
					Type:         slack.METPlainTextInput,
					ActionID:     "summary_text",
					InitialValue: currentSummary,
					Multiline:    true,
					Placeholder: slack.NewTextBlockObject(
						"plain_text", "例: ユーザーがログインできない", false, false,
					),
				},
				Optional: false,
			},
		},
	}
}
