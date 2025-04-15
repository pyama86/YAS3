package blocks

import "github.com/slack-go/slack"

func InChannelOptions() []*slack.OptionBlockObject {
	return []*slack.OptionBlockObject{
		slack.NewOptionBlockObject(
			"recovery_incident",
			slack.NewTextBlockObject("plain_text", "✅ 復旧の宣言を出す", false, false),
			nil,
		),
		slack.NewOptionBlockObject(
			"stop_timekeeper",
			slack.NewTextBlockObject("plain_text", "⏹️ タイムキーパーをとめる", false, false),
			nil,
		),
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
			"create_postmortem",
			slack.NewTextBlockObject("plain_text", "📝 ポストモーテムを作成する", false, false),
			nil,
		),
	}
}
