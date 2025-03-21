package blocks

import (
	"fmt"

	"github.com/slack-go/slack"
)

func IncidentReportRequest(userID string) []slack.Block {
	return []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				fmt.Sprintf("<@%s>さん、チャンネルの作成有難うございます。まずは現在のわかっている状況をチャンネルで報告してください。", userID),
				false,
				false,
			),
			nil,
			nil,
		),
		slack.NewRichTextBlock("事象内容を共有してください",
			slack.NewRichTextList(slack.RTEListOrdered, 0,
				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement("事象内容を共有してください", nil),
				),
				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement("ユーザー対応やお知らせは必要そうですか？", nil),
				),
				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement("原因はわかっていますか？", nil),
				),
				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement("復旧目処は立っていますか？", nil),
				),
				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement("インシデントレベルを設定してください。現在は影響なしに設定されています", nil),
				),
			),
		),
	}
}
