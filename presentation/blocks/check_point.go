package blocks

import (
	"fmt"

	"github.com/slack-go/slack"
)

func CheckPoint(elapsedStr string) []slack.Block {

	return []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				fmt.Sprintf(
					"15分ごとのチェックポイントです。インシデントの検知から *%s* 経過しています。", elapsedStr,
				),
				false,
				false,
			),
			nil,
			nil,
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				":loudspeaker: *状況更新アナウンス*\n\n事象内容やインシデントレベルの変更など状況に変化があれば、こちらで最新情報を共有してください。",
				false,
				false,
			),
			nil,
			nil,
		),
		slack.NewActionBlock("keeper_action",
			slack.NewOptionsSelectBlockElement(
				slack.OptTypeStatic,
				slack.NewTextBlockObject("plain_text", "操作を選択してください", false, false),
				"in_channel_options",
				InChannelOptions()...,
			),
			slack.NewButtonBlockElement(
				"progress_summary_action",
				"progress_summary_button",
				slack.NewTextBlockObject("plain_text", "📊 進捗サマリを作成", false, false),
			).WithStyle(slack.StylePrimary),
		),
	}

}
