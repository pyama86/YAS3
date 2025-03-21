package blocks

import (
	"fmt"

	"github.com/slack-go/slack"
)

func IncidentLevelChanged(userID, incidentLevel string) []slack.Block {
	return []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", "<!here> インシデントレベルが変更されました", false, false),
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
