package blocks

import (
	"fmt"

	"github.com/slack-go/slack"
)

func IncidentLevelChanged(userID, incidentLevel, notificationType string) []slack.Block {
	notificationText := AddNotification("インシデントレベルが変更されました", notificationType)

	return []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", notificationText, false, false),
			nil,
			nil,
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("<@%s> さんがインシデントレベルを「 *%s* 」に変更しました", userID, incidentLevel), false, false),
			nil,
			nil,
		),
	}
}
