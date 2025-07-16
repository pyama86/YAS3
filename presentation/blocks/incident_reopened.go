package blocks

import (
	"fmt"

	"github.com/pyama86/YAS3/domain/entity"
	"github.com/slack-go/slack"
)

func IncidentReopened(userID, handlerID string) []slack.Block {
	blocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("<@%s> さんがインシデントを再開しました", userID), false, false),
			nil, nil,
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", "<!here> インシデントが再発し、対応を再開します。", false, false),
			nil, nil,
		),
	}

	if handlerID != "" {
		blocks = append(blocks,
			slack.NewSectionBlock(
				slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("対応責任者: <@%s>", handlerID), false, false),
				nil, nil,
			),
		)
	}

	blocks = append(blocks,
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", ":hourglass_flowing_sand: タイムキーパーを再開しました", false, false),
			nil, nil,
		),
	)

	return blocks
}

func IncidentReopenedAnnounce(summaryText, incidentLevel, channelName string, service *entity.Service) []slack.Block {
	blocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", "🔴 インシデントが再発しました", false, false),
			nil, nil,
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*サービス:* %s", service.Name), false, false),
			nil, nil,
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*レベル:* %s", incidentLevel), false, false),
			nil, nil,
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*事象内容:* %s", summaryText), false, false),
			nil, nil,
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("対応チャンネル: <#%s>", channelName), false, false),
			nil, nil,
		),
	}

	return blocks
}
