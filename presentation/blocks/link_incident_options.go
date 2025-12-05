package blocks

import "github.com/slack-go/slack"

// LinkIncidentOptions は非インシデントチャンネル用のメニューオプション
func LinkIncidentOptions(isThread bool, isLinked bool) []*slack.OptionBlockObject {
	var options []*slack.OptionBlockObject

	// 未クローズインシデント一覧を最初に追加（常に表示）
	options = append(options, slack.NewOptionBlockObject(
		"list_open_incidents",
		slack.NewTextBlockObject("plain_text", "📋 未クローズインシデント一覧", false, false),
		nil,
	))

	if isLinked {
		// 既に紐づけられている場合は解除オプションのみ表示
		var unlinkText string
		if isThread {
			unlinkText = "🔓 このスレッドの紐づけを解除する"
		} else {
			unlinkText = "🔓 このチャンネルの紐づけを解除する"
		}

		options = append(options, slack.NewOptionBlockObject(
			"unlink_from_incident",
			slack.NewTextBlockObject("plain_text", unlinkText, false, false),
			nil,
		))
	} else {
		// 紐づけられていない場合は紐づけオプションのみ表示
		var linkText string
		if isThread {
			linkText = "🔗 インシデントとこのスレッドを紐づける"
		} else {
			linkText = "🔗 インシデントとこのチャンネルを紐づける"
		}

		options = append(options, slack.NewOptionBlockObject(
			"link_to_incident",
			slack.NewTextBlockObject("plain_text", linkText, false, false),
			nil,
		))
	}

	return options
}
