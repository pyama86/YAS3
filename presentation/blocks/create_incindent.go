package blocks

import (
	"fmt"

	"github.com/pyama86/YAS3/domain/entity"
	"github.com/slack-go/slack"
)

var UrgencyMap = map[string]string{
	"none":     "✅ サービスへの影響はない",
	"warning":  "🔍 サービスへの影響を調査する",
	"error":    "⚠️ サービスに影響が出ている",
	"critical": "🚨 緊急の対応を要する",
}

func CreateIncident(services []entity.Service) slack.Blocks {
	serviceOptions := make([]*slack.OptionBlockObject, 0, len(services))
	for _, service := range services {
		serviceOptions = append(serviceOptions, slack.NewOptionBlockObject(
			fmt.Sprintf("%d", service.ID),
			slack.NewTextBlockObject("plain_text", service.Name, false, false),
			nil,
		))
	}
	urgencyOptions := make([]*slack.OptionBlockObject, 0, len(UrgencyMap))
	for _, key := range []string{"critical", "error", "warning", "none"} {
		urgencyOptions = append(urgencyOptions, slack.NewOptionBlockObject(
			key,
			slack.NewTextBlockObject("plain_text", UrgencyMap[key], false, false), nil),
		)
	}

	return slack.Blocks{
		BlockSet: []slack.Block{
			// サービス
			&slack.InputBlock{
				Type:    slack.MBTInput,
				BlockID: "service_block",
				Label: &slack.TextBlockObject{
					Type: "plain_text",
					Text: "🛠️ サービス",
				},
				Element: &slack.SelectBlockElement{
					Type:        slack.OptTypeStatic,
					ActionID:    "service_select",
					Options:     serviceOptions,
					Placeholder: slack.NewTextBlockObject("plain_text", "選択してください", false, false),
				},
				Optional: false,
			},

			slack.NewDividerBlock(),

			// 事象内容
			&slack.InputBlock{
				Type:    slack.MBTInput,
				BlockID: "incident_summary_block",
				Label: &slack.TextBlockObject{
					Type: "plain_text",
					Text: "現在の状況について教えて下さい",
				},
				Element: &slack.PlainTextInputBlockElement{
					Type:      slack.METPlainTextInput,
					ActionID:  "summary_text",
					Multiline: true,
					Placeholder: slack.NewTextBlockObject(
						"plain_text", "例: ユーザーがログインできない", false, false,
					),
				},
				Optional: false,
			},

			slack.NewDividerBlock(),

			// 緊急度
			&slack.InputBlock{
				Type:    slack.MBTInput,
				BlockID: "urgency_block",
				Label: &slack.TextBlockObject{
					Type: "plain_text",
					Text: "⚠️ 緊急度",
				},
				Element: &slack.SelectBlockElement{
					Type:        slack.OptTypeStatic,
					ActionID:    "urgency_select",
					Options:     urgencyOptions,
					Placeholder: slack.NewTextBlockObject("plain_text", "選択してください", false, false),
				},
				Optional: false,
			},
		},
	}

}
