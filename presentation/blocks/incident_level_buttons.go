package blocks

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pyama86/YAS3/domain/entity"
	"github.com/slack-go/slack"
)

func IncidentLevelButtons(levels []entity.IncidentLevel) []slack.Block {
	// IncidentLevel を昇順にソート
	sortedLevels := make([]entity.IncidentLevel, len(levels))
	copy(sortedLevels, levels)
	sort.Slice(sortedLevels, func(i, j int) bool {
		return sortedLevels[i].Level < sortedLevels[j].Level
	})

	// 説明文のセクションブロックと区切りブロック
	blocks := []slack.Block{
		slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", "📢 インシデントレベルを選択してください！", false, false)),
		slack.NewDividerBlock(),
		// 固定のレベル0（サービス影響なし）ブロック
		&slack.SectionBlock{
			Type:    slack.MBTSection,
			BlockID: "section-0",
			Text: &slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: "*インシデントレベル:0* 🙆 サービス影響なし",
			},
			Accessory: &slack.Accessory{
				ButtonElement: &slack.ButtonBlockElement{
					Type:     slack.METButton,
					ActionID: "incident_level_button",
					Value:    "0",
					Text: &slack.TextBlockObject{
						Type: "plain_text",
						Text: "選択",
					},
					Style: slack.StylePrimary,
				},
			},
		},
	}

	// レベル0以外の各 IncidentLevel のブロックを追加
	for _, level := range sortedLevels {
		if level.Level == 0 {
			continue
		}
		fireEmojis := strings.Repeat("🔥", level.Level)
		text := fmt.Sprintf("*インシデントレベル%d:* %s %s", level.Level, fireEmojis, level.Description)
		block := &slack.SectionBlock{
			Type:    slack.MBTSection,
			BlockID: fmt.Sprintf("section-%d", level.Level),
			Text: &slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: text,
			},
			Accessory: &slack.Accessory{
				ButtonElement: &slack.ButtonBlockElement{
					Type:     slack.METButton,
					ActionID: "incident_level_button",
					Value:    fmt.Sprintf("%d", level.Level),
					Text: &slack.TextBlockObject{
						Type: "plain_text",
						Text: "選択",
					},
					Style: slack.StyleDanger,
				},
			},
		}
		blocks = append(blocks, block)
	}
	return blocks
}
