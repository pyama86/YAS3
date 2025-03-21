package blocks

import "github.com/slack-go/slack"

func HandlerRecruitmentMessage() []slack.Block {
	return []slack.Block{
		slack.NewHeaderBlock(
			slack.NewTextBlockObject("plain_text", "🚨 ハンドラー募集！", false, false),
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", "👩‍💻 インシデント対応を指揮するハンドラーを決めます。\n状況把握と指揮をお願いします！", false, false),
			nil,
			nil,
		),
		slack.NewDividerBlock(),
		slack.NewActionBlock(
			"handler_action",
			slack.NewButtonBlockElement(
				"handler_button",
				"handler_button",
				slack.NewTextBlockObject("plain_text", "👋 ハンドラーは私です！", false, false),
			).WithStyle(slack.StylePrimary),
		),
		slack.NewContextBlock("handler_context", []slack.MixedElement{
			slack.NewTextBlockObject("mrkdwn", "※質問がある場合はこのチャンネルでお知らせください。", false, false),
		}...),
	}
}
