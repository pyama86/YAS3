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
		// リッチテキストブロックで番号付きリストを表示
		slack.NewRichTextBlock("ハンドラの責務",
			slack.NewRichTextList(slack.RTEListOrdered, 0,
				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement("状況を迅速に把握し、初動対応を行う", nil),
				),
				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement("事象レベルを判断し、設定する", nil),
				),

				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement("チームに明確な指示を出し、統率を取る", nil),
				),
				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement("最新情報を適時共有し、ステークホルダーと連携する", nil),
				),
				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement("問題解決に向けたアクションプランを策定する", nil),
				),
				slack.NewRichTextSection(
					slack.NewRichTextSectionTextElement("進捗状況を継続的に報告し、必要な支援を要請する", nil),
				),
			),
		),
	}
}
