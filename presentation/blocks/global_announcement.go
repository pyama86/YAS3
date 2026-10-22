package blocks

import (
	"fmt"

	"github.com/pyama86/YAS3/domain/entity"
	"github.com/slack-go/slack"
)

// GlobalAnnounceLoading はAIが下書きを生成している間に表示するローディングモーダルのブロック
func GlobalAnnounceLoading() slack.Blocks {
	return slack.Blocks{
		BlockSet: []slack.Block{
			slack.NewSectionBlock(
				slack.NewTextBlockObject(
					"mrkdwn",
					":hourglass_flowing_sand: AIが障害報告の下書きを作成しています…\nしばらくお待ちください。",
					false,
					false,
				),
				nil,
				nil,
			),
		},
	}
}

// GlobalAnnounceForm は障害報告の入力フォーム。AIが生成した下書きを初期値として埋める
// 各入力には文字数上限を設定し、通知時の文字数制限超過を防ぐ
func GlobalAnnounceForm(summary, occurredAt, impact string) slack.Blocks {
	return slack.Blocks{
		BlockSet: []slack.Block{
			&slack.InputBlock{
				Type:    slack.MBTInput,
				BlockID: "summary_block",
				Label:   slack.NewTextBlockObject("plain_text", "発生事象", false, false),
				Element: &slack.PlainTextInputBlockElement{
					Type:         slack.METPlainTextInput,
					ActionID:     "summary_text",
					InitialValue: summary,
					Multiline:    true,
					MaxLength:    2000,
					Placeholder:  slack.NewTextBlockObject("plain_text", "例: ◯◯機能でエラーが発生しています", false, false),
				},
				Optional: false,
			},
			&slack.InputBlock{
				Type:    slack.MBTInput,
				BlockID: "occurred_block",
				Label:   slack.NewTextBlockObject("plain_text", "発生日時", false, false),
				Element: &slack.PlainTextInputBlockElement{
					Type:         slack.METPlainTextInput,
					ActionID:     "occurred_text",
					InitialValue: occurredAt,
					MaxLength:    100,
					Placeholder:  slack.NewTextBlockObject("plain_text", "例: 2026-06-09 14:30 頃", false, false),
				},
				Optional: false,
			},
			&slack.InputBlock{
				Type:    slack.MBTInput,
				BlockID: "impact_block",
				Label:   slack.NewTextBlockObject("plain_text", "影響範囲", false, false),
				Element: &slack.PlainTextInputBlockElement{
					Type:         slack.METPlainTextInput,
					ActionID:     "impact_text",
					InitialValue: impact,
					Multiline:    true,
					MaxLength:    2000,
					Placeholder:  slack.NewTextBlockObject("plain_text", "例: 一部ユーザーがログインできません", false, false),
				},
				Optional: true,
			},
		},
	}
}

// GlobalAnnouncement は障害報告メッセージ（attachment内）のブロックを組み立てる
func GlobalAnnouncement(summary, occurredAt, impact, channelID string, service *entity.Service) []slack.Block {
	fields := []*slack.TextBlockObject{
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*発生日時:* %s", truncateRunes(occurredAt, 300)), false, false),
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*対応チャンネル:* <#%s>", channelID), false, false),
	}
	if service != nil {
		fields = append(fields, slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*サービス:* %s", service.Name), false, false))
	}

	blockSet := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", ":rotating_light: *障害報告*", false, false),
			nil,
			nil,
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*発生事象:*\n%s", truncateRunes(summary, 2800)), false, false),
			nil,
			nil,
		),
		slack.NewSectionBlock(nil, fields, nil),
	}
	if impact != "" {
		blockSet = append(blockSet, slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*影響範囲:*\n%s", truncateRunes(impact, 2800)), false, false),
			nil,
			nil,
		))
	}
	return blockSet
}

// truncateRunes はSlackのmrkdwnセクション上限(3000文字)に収まるようルーン単位で切り詰める
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
