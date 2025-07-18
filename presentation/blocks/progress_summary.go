package blocks

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/slack-go/slack"
)

func ProgressSummary(summary string) []slack.Block {
	// マークダウンサマリをSlackブロックに変換
	blocks := parseProgressSummaryToBlocks(summary)

	// 区切り線とボタンを追加
	blocks = append(blocks, slack.NewDividerBlock())
	blocks = append(blocks, slack.NewActionBlock(
		"report_post_action",
		slack.NewButtonBlockElement(
			"report_post_action",
			"report_post_button",
			slack.NewTextBlockObject("plain_text", "📢 報告chに投稿", false, false),
		).WithStyle(slack.StylePrimary),
	))

	return blocks
}

// マークダウンサマリをSlackブロックに変換する関数
func parseProgressSummaryToBlocks(summary string) []slack.Block {
	var blocks []slack.Block

	// メインヘッダーを追加
	blocks = append(blocks, slack.NewHeaderBlock(
		slack.NewTextBlockObject("plain_text", "📊 進捗サマリ", false, false),
	))

	// セクションに分割（### で始まる行で分割）
	sections := regexp.MustCompile(`(?m)^### `).Split(summary, -1)

	for i, section := range sections {
		if i == 0 && strings.TrimSpace(section) == "" {
			continue // 最初の空セクションをスキップ
		}

		lines := strings.Split(strings.TrimSpace(section), "\n")
		if len(lines) == 0 {
			continue
		}

		// セクションタイトルを取得
		title := strings.TrimSpace(lines[0])
		if title == "" {
			continue
		}

		// セクションタイトルをヘッダーとして追加
		blocks = append(blocks, slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*%s*", title), false, false),
			nil,
			nil,
		))

		// セクションの内容を処理
		if len(lines) > 1 {
			content := strings.Join(lines[1:], "\n")
			content = strings.TrimSpace(content)

			if content != "" {
				// リスト項目を整形
				content = formatListItems(content)

				blocks = append(blocks, slack.NewSectionBlock(
					slack.NewTextBlockObject("mrkdwn", content, false, false),
					nil,
					nil,
				))
			}
		}
	}

	return blocks
}

// リスト項目を Slack 用に整形
func formatListItems(content string) string {
	lines := strings.Split(content, "\n")
	var formattedLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// マークダウンのリスト項目（- で始まる）をSlack用に変換
		if strings.HasPrefix(line, "- ") {
			// "- " を "• " に変換してインデント
			line = "• " + strings.TrimPrefix(line, "- ")
		}

		// **太字** を *太字* に変換（Slack用）
		line = regexp.MustCompile(`\*\*(.*?)\*\*`).ReplaceAllString(line, "*$1*")

		formattedLines = append(formattedLines, line)
	}

	return strings.Join(formattedLines, "\n")
}

func ProgressSummaryLoading() []slack.Block {
	return []slack.Block{
		slack.NewHeaderBlock(
			slack.NewTextBlockObject("plain_text", "🔄 進捗サマリを作成中...", false, false),
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				"AIが現在のインシデント対応状況を分析しています。しばらくお待ちください...",
				false,
				false,
			),
			nil,
			nil,
		),
	}
}

func ReportPostSuccess(reportChannel string) []slack.Block {
	return []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				"mrkdwn",
				fmt.Sprintf("✅ 進捗サマリを %s に投稿しました", reportChannel),
				false,
				false,
			),
			nil,
			nil,
		),
	}
}
