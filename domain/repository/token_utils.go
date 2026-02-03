package repository

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkoukk/tiktoken-go"
	"github.com/slack-go/slack"
)

const (
	// デフォルトのトークン制限（大規模コンテキストモデル向け）
	DefaultMaxTokens = 200000
	// 1つのメッセージあたりの平均トークン数の見積もり
	AverageTokensPerMessage = 30
)

// GetMaxTokens は環境変数またはデフォルト値からトークン制限を取得
func GetMaxTokens() int {
	if envMaxTokens := os.Getenv("MAX_TOKENS"); envMaxTokens != "" {
		if maxTokens, err := strconv.Atoi(envMaxTokens); err == nil && maxTokens > 0 {
			return maxTokens
		}
	}
	return DefaultMaxTokens
}

// トークン計算ユーティリティ
type TokenCalculator struct {
	encoder *tiktoken.Tiktoken
}

// 新しいトークン計算機を作成
func NewTokenCalculator() (*TokenCalculator, error) {
	encoder, err := tiktoken.EncodingForModel("gpt-4")
	if err != nil {
		return nil, fmt.Errorf("failed to get encoding for GPT-4: %w", err)
	}

	return &TokenCalculator{
		encoder: encoder,
	}, nil
}

// テキストのトークン数を計算
func (tc *TokenCalculator) CountTokens(text string) int {
	if tc.encoder == nil {
		// フォールバック: 文字数 / 4 (おおよその見積もり)
		return len(text) / 4
	}

	tokens := tc.encoder.Encode(text, nil, nil)
	return len(tokens)
}

// Slackメッセージをテキストに変換
func (tc *TokenCalculator) FormatMessage(msg slack.Message) string {
	timestamp, _ := strconv.ParseFloat(msg.Timestamp, 64)
	t := time.Unix(int64(timestamp), 0)

	// メッセージの基本形式: "2006-01-02 15:04:05 ユーザー名: メッセージ内容"
	text := fmt.Sprintf("%s %s: %s",
		t.Format("2006-01-02 15:04:05"),
		msg.User,
		msg.Text)

	// スレッドの場合は分かりやすくする
	if msg.ThreadTimestamp != "" && msg.ThreadTimestamp != msg.Timestamp {
		text = "  └ " + text // インデントでスレッド表示
	}

	return text
}

// メッセージリストのトータルトークン数を計算
func (tc *TokenCalculator) CountMessagesTokens(messages []slack.Message, basePrompt string) int {
	var allText strings.Builder
	allText.WriteString(basePrompt)
	allText.WriteString("\n\n")

	for _, msg := range messages {
		allText.WriteString(tc.FormatMessage(msg))
		allText.WriteString("\n")
	}

	return tc.CountTokens(allText.String())
}

// メッセージを適切なサイズに分割
func (tc *TokenCalculator) SplitMessages(messages []slack.Message, basePrompt string, maxTokens int) [][]slack.Message {
	if len(messages) == 0 {
		return [][]slack.Message{}
	}

	var chunks [][]slack.Message
	var currentChunk []slack.Message
	baseTokens := tc.CountTokens(basePrompt)
	currentTokens := baseTokens

	for _, msg := range messages {
		msgText := tc.FormatMessage(msg)
		msgTokens := tc.CountTokens(msgText)

		// 現在のチャンクに追加すると制限を超える場合
		if currentTokens+msgTokens > maxTokens && len(currentChunk) > 0 {
			chunks = append(chunks, currentChunk)
			currentChunk = []slack.Message{msg}
			currentTokens = baseTokens + msgTokens
		} else {
			currentChunk = append(currentChunk, msg)
			currentTokens += msgTokens
		}
	}

	// 最後のチャンクを追加
	if len(currentChunk) > 0 {
		chunks = append(chunks, currentChunk)
	}

	return chunks
}

// 重要なメッセージを優先的に保持する分割
func (tc *TokenCalculator) SplitMessagesWithPriority(messages []slack.Message, basePrompt string, maxTokens int) [][]slack.Message {
	// まず重要度でソート（ピン留め、長いメッセージ、スレッドの親投稿など）
	prioritizedMessages := tc.prioritizeMessages(messages)

	return tc.SplitMessages(prioritizedMessages, basePrompt, maxTokens)
}

// メッセージの重要度を判定して並び替え
func (tc *TokenCalculator) prioritizeMessages(messages []slack.Message) []slack.Message {
	var important, normal []slack.Message

	for _, msg := range messages {
		if tc.isImportantMessage(msg) {
			important = append(important, msg)
		} else {
			normal = append(normal, msg)
		}
	}

	// 重要なメッセージを先頭に配置
	result := make([]slack.Message, 0, len(messages))
	result = append(result, important...)
	result = append(result, normal...)

	return result
}

// メッセージが重要かどうかを判定
func (tc *TokenCalculator) isImportantMessage(msg slack.Message) bool {
	// 長いメッセージ（詳細な情報が含まれている可能性）
	if len(msg.Text) > 200 {
		return true
	}

	// スレッドの親投稿
	if msg.ThreadTimestamp != "" && msg.ThreadTimestamp == msg.Timestamp {
		return true
	}

	// 特定のキーワードを含む
	importantKeywords := []string{"解決", "復旧", "原因", "対応", "エラー", "障害", "修正"}
	msgText := strings.ToLower(msg.Text)
	for _, keyword := range importantKeywords {
		if strings.Contains(msgText, keyword) {
			return true
		}
	}

	return false
}

// 分割されたサマリを統合するためのプロンプト生成
func (tc *TokenCalculator) CreateMergePrompt(summaries []string) string {
	var builder strings.Builder
	builder.WriteString("以下は複数の部分的なインシデント進捗サマリです。これらを統合して一つの完全なサマリを作成してください：\n\n")

	for i, summary := range summaries {
		builder.WriteString(fmt.Sprintf("## 部分サマリ %d\n%s\n\n", i+1, summary))
	}

	builder.WriteString("これらの情報を統合し、重複を排除して、以下の構成で最終サマリを作成してください：\n")
	builder.WriteString("### 📊 インシデント概要\n### 🔄 現在の状況\n### ✅ 実施済み対応\n### 🎯 次のアクション\n### 📢 関係者への情報")

	return builder.String()
}
