package blocks

import "github.com/slack-go/slack"

// 進捗サマリ作成の確認フォーム
func ProgressSummaryConfirmation() []slack.Block {
	return []slack.Block{
		slack.NewHeaderBlock(
			slack.NewTextBlockObject("plain_text", "📊 進捗サマリ作成の確認", false, false),
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				"現在のインシデント対応状況を分析して進捗サマリを作成します。\n処理には数十秒かかる場合があります。\n\n実行してもよろしいですか？",
				false,
				false,
			),
			nil,
			nil,
		),
		slack.NewDividerBlock(),
		slack.NewActionBlock(
			"progress_summary_confirm",
			slack.NewButtonBlockElement(
				"progress_summary_execute",
				"confirm",
				slack.NewTextBlockObject("plain_text", "✅ 実行する", false, false),
			).WithStyle(slack.StylePrimary),
			slack.NewButtonBlockElement(
				"progress_summary_cancel",
				"cancel",
				slack.NewTextBlockObject("plain_text", "❌ キャンセル", false, false),
			),
		),
	}
}

// 復旧宣言の確認フォーム
func RecoveryConfirmation() []slack.Block {
	return []slack.Block{
		slack.NewHeaderBlock(
			slack.NewTextBlockObject("plain_text", "✅ 復旧宣言の確認", false, false),
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				"⚠️ *重要な操作です* ⚠️\n\nインシデントの復旧を宣言します。\n以下の処理が実行されます：\n\n• タイムキーパーが停止されます\n• アナウンスチャンネルに復旧通知が送信されます\n• チャンネルトピックに【復旧】が追加されます\n\n本当に復旧宣言を行いますか？",
				false,
				false,
			),
			nil,
			nil,
		),
		slack.NewDividerBlock(),
		slack.NewActionBlock(
			"recovery_confirm",
			slack.NewButtonBlockElement(
				"recovery_execute",
				"confirm",
				slack.NewTextBlockObject("plain_text", "✅ 復旧宣言を実行", false, false),
			).WithStyle(slack.StylePrimary),
			slack.NewButtonBlockElement(
				"recovery_cancel",
				"cancel",
				slack.NewTextBlockObject("plain_text", "❌ キャンセル", false, false),
			),
		),
	}
}

// タイムキーパー停止の確認フォーム
func TimekeeperStopConfirmation() []slack.Block {
	return []slack.Block{
		slack.NewHeaderBlock(
			slack.NewTextBlockObject("plain_text", "⏹️ タイムキーパー停止の確認", false, false),
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				"タイムキーパーを停止します。\n\n15分間隔のチェックポイントメッセージが送信されなくなります。\n（インシデント再開で再度有効化されます）\n\n停止してもよろしいですか？",
				false,
				false,
			),
			nil,
			nil,
		),
		slack.NewDividerBlock(),
		slack.NewActionBlock(
			"timekeeper_stop_confirm",
			slack.NewButtonBlockElement(
				"timekeeper_stop_execute",
				"confirm",
				slack.NewTextBlockObject("plain_text", "⏹️ 停止する", false, false),
			).WithStyle(slack.StyleDanger),
			slack.NewButtonBlockElement(
				"timekeeper_stop_cancel",
				"cancel",
				slack.NewTextBlockObject("plain_text", "❌ キャンセル", false, false),
			),
		),
	}
}
