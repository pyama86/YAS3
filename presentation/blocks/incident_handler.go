package blocks

import (
	"fmt"

	"github.com/slack-go/slack"
)

func AcceptIncidentHandler(userID string) []slack.Block {
	return []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				fmt.Sprintf(
					":wave: <@%s> さん、ハンドラになっていただきありがとうございます！\n\nこれからインシデントの指揮をお願いします。以下は、ハンドラとして求められる主な責務です",
					userID,
				),
				false,
				false,
			),
			nil,
			nil,
		),
		slack.NewHeaderBlock(
			slack.NewTextBlockObject("plain_text", "☀️  ハンドラの役割", false, false),
		),
		slack.NewRichTextBlock("ハンドラの役割",
			slack.NewRichTextList(slack.RTEListOrdered, 0,
				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement("1. 状況を迅速に把握し、初動対応を行う。", nil),
				),
				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement("2. 事象レベルを判断し、設定する", nil),
				),

				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement("3. 調査や作業を行う作業者を招集してください", nil),
				),
				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement("4. 作業者に明確な指示を出し、統率を取る", nil),
				),
				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement("5. 最新情報を適時共有し、ステークホルダーと連携する", nil),
				),
				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement("6. 問題解決に向けたアクションプランを策定する", nil),
				),
			),
		),
	}
}
