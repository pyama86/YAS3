package blocks

import "github.com/slack-go/slack"

func InChannelOptions(manualGlobalAnnounce bool) []*slack.OptionBlockObject {
	options := []*slack.OptionBlockObject{
		slack.NewOptionBlockObject(
			"set_incident_level",
			slack.NewTextBlockObject("plain_text", "⚙️ 事象レベルをセットする", false, false),
			nil,
		),
		slack.NewOptionBlockObject(
			"edit_incident_summary",
			slack.NewTextBlockObject("plain_text", "📝 事象内容を編集する", false, false),
			nil,
		),
		slack.NewOptionBlockObject(
			"create_progress_summary",
			slack.NewTextBlockObject("plain_text", "📊 進捗サマリを作成する", false, false),
			nil,
		),
		slack.NewOptionBlockObject(
			"stop_timekeeper",
			slack.NewTextBlockObject("plain_text", "⏹️ タイムキーパーをとめる", false, false),
			nil,
		),
		slack.NewOptionBlockObject(
			"recovery_incident",
			slack.NewTextBlockObject("plain_text", "✅ 復旧の宣言を出す", false, false),
			nil,
		),
		slack.NewOptionBlockObject(
			"create_postmortem",
			slack.NewTextBlockObject("plain_text", "📝 ポストモーテムを作成する", false, false),
			nil,
		),
		slack.NewOptionBlockObject(
			"reopen_incident",
			slack.NewTextBlockObject("plain_text", "🔴 インシデントを再開する", false, false),
			nil,
		),
	}
	if manualGlobalAnnounce {
		options = append(options, slack.NewOptionBlockObject(
			"announce_global",
			slack.NewTextBlockObject("plain_text", "📢 障害報告を送る", false, false),
			nil,
		))
	}
	return options
}
