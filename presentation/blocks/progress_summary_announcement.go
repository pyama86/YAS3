package blocks

import (
	"fmt"
	"regexp"

	"github.com/pyama86/YAS3/domain/entity"
	"github.com/slack-go/slack"
)

func ProgressSummaryAnnouncement(summary, incidentChannelID string, service *entity.Service) []slack.Block {
	serviceName := "不明"
	if service != nil {
		serviceName = service.Name
	}

	return []slack.Block{
		slack.NewHeaderBlock(
			slack.NewTextBlockObject("plain_text", "📊 インシデント進捗サマリ", false, false),
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				fmt.Sprintf("*サービス*: %s\n*インシデントチャンネル*: <#%s>", serviceName, incidentChannelID),
				false,
				false,
			),
			nil,
			nil,
		),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				convertMarkdownToSlack(summary),
				false,
				false,
			),
			nil,
			nil,
		),
	}
}

// マークダウンの**太字**をSlackの*太字*に変換
func convertMarkdownToSlack(text string) string {
	return regexp.MustCompile(`\*\*(.*?)\*\*`).ReplaceAllString(text, "*$1*")
}
