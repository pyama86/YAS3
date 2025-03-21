package blocks

import (
	"fmt"
	"strings"

	"github.com/pyama86/YAS3/domain/entity"
	"github.com/slack-go/slack"
)

func InviteMembers(service *entity.Service) []slack.Block {

	headerBlock := slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", "📢 インシデント対応チーム召喚！", false, false))

	messageSection := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", "インシデント対応チームが招集されました。\n速やかに対応を開始してください！", false, false), nil, nil)

	membersText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*招集メンバー:*\n%s", strings.Join(service.IncidentTeamMembers, ", ")), false, false)
	membersSection := slack.NewSectionBlock(membersText, nil, nil)

	return []slack.Block{
		headerBlock,
		messageSection,
		slack.NewDividerBlock(),
		membersSection,
	}

}
