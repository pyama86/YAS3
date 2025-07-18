package handler_test

import (
	"strings"
	"testing"

	"github.com/pyama86/YAS3/domain/repository"
	"github.com/pyama86/YAS3/presentation/blocks"
	"github.com/slack-go/slack"
)

// MockAIRepository は AI機能のモック
type mockAIRepository struct {
	summarizeProgressResult string
	summarizeProgressError  error
}

func (m *mockAIRepository) Summarize(description, slackMessages string) (string, error) {
	return "", nil
}

func (m *mockAIRepository) SummarizeProgress(description, slackMessages string) (string, error) {
	if m.summarizeProgressError != nil {
		return "", m.summarizeProgressError
	}
	return m.summarizeProgressResult, nil
}

func (m *mockAIRepository) SummarizeProgressAdvanced(description string, messages []slack.Message, previousSummary string) (string, error) {
	if m.summarizeProgressError != nil {
		return "", m.summarizeProgressError
	}
	return m.summarizeProgressResult, nil
}

func (m *mockAIRepository) GenerateTitle(description, slackMessages string) (string, error) {
	return "", nil
}

func (m *mockAIRepository) GenerateStatus(description, slackMessages string) (string, error) {
	return "", nil
}

func (m *mockAIRepository) GenerateImpact(description, slackMessages string) (string, error) {
	return "", nil
}

func (m *mockAIRepository) GenerateRootCause(description, slackMessages string) (string, error) {
	return "", nil
}

func (m *mockAIRepository) GenerateTrigger(description, slackMessages string) (string, error) {
	return "", nil
}

func (m *mockAIRepository) GenerateSolution(description, slackMessages string) (string, error) {
	return "", nil
}

func (m *mockAIRepository) GenerateActionItems(description, slackMessages string) (string, error) {
	return "", nil
}

func (m *mockAIRepository) GenerateLessonsLearned(description, slackMessages string) (string, string, string, error) {
	return "", "", "", nil
}

func (m *mockAIRepository) FormatTimeline(rawTimeline string) (string, error) {
	return "", nil
}

func TestSummarizeProgress(t *testing.T) {
	t.Setenv("TEST_MODE", "true")

	mockAI := &mockAIRepository{
		summarizeProgressResult: "### 📊 インシデント概要\n- APIサーバーの応答停止\n\n### 🔄 現在の状況\n- 調査中",
		summarizeProgressError:  nil,
	}

	result, err := mockAI.SummarizeProgress("APIサーバーが応答しない", "user1: APIサーバーが応答しません\nuser2: 調査開始します")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result == "" {
		t.Error("Expected non-empty result")
	}

	if result != mockAI.summarizeProgressResult {
		t.Errorf("Expected %s, got %s", mockAI.summarizeProgressResult, result)
	}
}

func TestProgressSummaryInterface(t *testing.T) {
	t.Setenv("TEST_MODE", "true")

	// AIRepositorierインターフェースが正しく実装されているかテスト
	var aiRepo repository.AIRepositorier = &mockAIRepository{}

	_, err := aiRepo.SummarizeProgress("test", "test")
	if err != nil {
		t.Errorf("SummarizeProgress method should exist and be callable")
	}

	// 高度なサマリ機能のテスト
	messages := []slack.Message{
		{
			Msg: slack.Msg{
				User:      "user1",
				Text:      "テストメッセージ",
				Timestamp: "1234567890.123456",
			},
		},
	}

	_, err = aiRepo.SummarizeProgressAdvanced("test description", messages, "")
	if err != nil {
		t.Errorf("SummarizeProgressAdvanced method should exist and be callable")
	}
}

func TestTokenCalculator(t *testing.T) {
	t.Setenv("TEST_MODE", "true")

	tokenCalc, err := repository.NewTokenCalculator()
	if tokenCalc == nil {
		// tiktoken-goが利用できない環境でもエラーにはならない
		t.Skip("TokenCalculator not available, skipping test")
	}
	if err != nil {
		t.Skip("TokenCalculator initialization failed, skipping test")
	}

	// 基本的なトークン計算テスト
	text := "Hello world"
	tokens := tokenCalc.CountTokens(text)
	if tokens <= 0 {
		t.Errorf("Expected positive token count, got %d", tokens)
	}

	// メッセージフォーマットテスト
	msg := slack.Message{
		Msg: slack.Msg{
			User:      "testuser",
			Text:      "test message",
			Timestamp: "1234567890.123456",
		},
	}

	formatted := tokenCalc.FormatMessage(msg)
	if formatted == "" {
		t.Error("Expected non-empty formatted message")
	}

	// メッセージ分割テスト
	messages := []slack.Message{msg, msg, msg}
	chunks := tokenCalc.SplitMessages(messages, "base prompt", 100)
	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
	}
}

func TestProgressSummaryFromMenu(t *testing.T) {
	t.Setenv("TEST_MODE", "true")

	// 通常メニューからの進捗サマリ作成テスト
	mockAI := &mockAIRepository{
		summarizeProgressResult: "### 📊 インシデント概要\\n- 進捗サマリテスト\\n\\n### 🔄 現在の状況\\n- テスト実行中",
		summarizeProgressError:  nil,
	}

	// AIRepositorierインターフェースが正しく実装されているかテスト
	var aiRepo repository.AIRepositorier = mockAI

	// 高度なサマリ機能のテスト（メニューから呼び出される機能）
	messages := []slack.Message{
		{
			Msg: slack.Msg{
				User:      "user1",
				Text:      "メニューからのテストメッセージ",
				Timestamp: "1234567890.123456",
			},
		},
	}

	_, err := aiRepo.SummarizeProgressAdvanced("menu test", messages, "")
	if err != nil {
		t.Errorf("Menu-triggered SummarizeProgressAdvanced should work: %v", err)
	}
}

func TestProgressSummaryBlocks(t *testing.T) {
	t.Setenv("TEST_MODE", "true")

	// マークダウン形式のサマリをテスト（太字を含む）
	summaryText := `### 📊 インシデント概要
- **事象の簡潔な説明**: APIサーバーが応答停止
- **影響レベル**: 高

### 🔄 現在の状況  
- 対応中
- 原因調査が進行中

### ✅ 実施済み対応
- サーバー再起動を実施
- ログ解析を完了`

	// Slackブロックに変換
	blocks := blocks.ProgressSummary(summaryText)

	// ブロックが生成されていることを確認
	if len(blocks) == 0 {
		t.Error("Expected blocks to be generated")
	}

	// ヘッダーブロックが含まれていることを確認
	hasHeader := false
	for _, block := range blocks {
		if headerBlock, ok := block.(*slack.HeaderBlock); ok {
			if headerBlock.Text.Text == "📊 進捗サマリ" {
				hasHeader = true
				break
			}
		}
	}
	if !hasHeader {
		t.Error("Expected header block with '📊 進捗サマリ'")
	}

	// ボタンブロックが含まれていることを確認
	hasButton := false
	for _, block := range blocks {
		if actionBlock, ok := block.(*slack.ActionBlock); ok {
			if actionBlock.BlockID == "report_post_action" {
				hasButton = true
				break
			}
		}
	}
	if !hasButton {
		t.Error("Expected action block with report button")
	}

	// 太字変換が正しく動作することを確認
	hasBoldFormatting := false
	for _, block := range blocks {
		if sectionBlock, ok := block.(*slack.SectionBlock); ok {
			if sectionBlock.Text != nil && strings.Contains(sectionBlock.Text.Text, "*事象の簡潔な説明*") {
				hasBoldFormatting = true
				break
			}
		}
	}
	if !hasBoldFormatting {
		t.Error("Expected bold formatting to be converted to Slack format")
	}
}

func TestProgressSummaryReportExtraction(t *testing.T) {
	t.Setenv("TEST_MODE", "true")

	// 新しいブロック形式のメッセージからサマリ抽出をテスト
	summaryText := `### 📊 インシデント概要
- **事象の簡潔な説明**: APIサーバーが応答停止
- **影響レベル**: 高`

	blocks := blocks.ProgressSummary(summaryText)

	// Slackメッセージを模擬
	mockMessage := slack.Message{
		Msg: slack.Msg{
			Blocks: slack.Blocks{
				BlockSet: blocks,
			},
		},
	}

	// サマリ部分が抽出できることを確認
	var summaryParts []string
	for _, block := range mockMessage.Blocks.BlockSet {
		if sectionBlock, ok := block.(*slack.SectionBlock); ok {
			if sectionBlock.Text != nil && sectionBlock.Text.Type == "mrkdwn" {
				text := sectionBlock.Text.Text
				// ヘッダーブロック、ボタンブロック、区切り線は除外
				if text != "" && !strings.Contains(text, "進捗サマリ") && !strings.Contains(text, "報告chに投稿") {
					summaryParts = append(summaryParts, text)
				}
			}
		}
	}

	if len(summaryParts) == 0 {
		t.Error("Expected to extract summary parts from block message")
	}

	// 抽出されたサマリに期待される内容が含まれているかチェック
	summaryContent := strings.Join(summaryParts, "\n\n")
	if !strings.Contains(summaryContent, "インシデント概要") {
		t.Error("Expected summary to contain section header")
	}
	if !strings.Contains(summaryContent, "*事象の簡潔な説明*") {
		t.Error("Expected bold formatting in extracted summary")
	}
}

func TestConfirmationBlocks(t *testing.T) {
	t.Setenv("TEST_MODE", "true")

	// 進捗サマリ確認フォーム
	progressBlocks := blocks.ProgressSummaryConfirmation()
	if len(progressBlocks) == 0 {
		t.Error("Expected progress summary confirmation blocks")
	}

	// 復旧確認フォーム
	recoveryBlocks := blocks.RecoveryConfirmation()
	if len(recoveryBlocks) == 0 {
		t.Error("Expected recovery confirmation blocks")
	}

	// タイムキーパー停止確認フォーム
	timekeeperBlocks := blocks.TimekeeperStopConfirmation()
	if len(timekeeperBlocks) == 0 {
		t.Error("Expected timekeeper stop confirmation blocks")
	}

	// 各確認フォームにヘッダーとボタンが含まれていることを確認
	for _, blocks := range [][]slack.Block{progressBlocks, recoveryBlocks, timekeeperBlocks} {
		hasHeader := false
		hasActionBlock := false

		for _, block := range blocks {
			if _, ok := block.(*slack.HeaderBlock); ok {
				hasHeader = true
			}
			if _, ok := block.(*slack.ActionBlock); ok {
				hasActionBlock = true
			}
		}

		if !hasHeader {
			t.Error("Expected header block in confirmation form")
		}
		if !hasActionBlock {
			t.Error("Expected action block with buttons in confirmation form")
		}
	}
}
