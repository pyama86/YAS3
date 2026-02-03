package repository

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Songmu/retry"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
	"github.com/openai/openai-go/option"
	"github.com/slack-go/slack"
)

type AIRepositorier interface {
	Summarize(description, slackMessages string) (string, error)
	SummarizeProgress(description, slackMessages string) (string, error)
	SummarizeProgressAdvanced(description string, messages []slack.Message, previousSummary string) (string, error)
	GenerateTitle(description, slackMessages string) (string, error)
	GenerateStatus(description, slackMessages string) (string, error)
	GenerateImpact(description, slackMessages string) (string, error)
	GenerateRootCause(description, slackMessages string) (string, error)
	GenerateTrigger(description, slackMessages string) (string, error)
	GenerateSolution(description, slackMessages string) (string, error)
	GenerateActionItems(description, slackMessages string) (string, error)
	GenerateLessonsLearned(description, slackMessages string) (string, string, string, error) // うまくいったこと、うまくいかなかったこと、幸運だったこと
	FormatTimeline(rawTimeline string) (string, error)
	AnalyzeRemainingTasks(description, slackMessages string) (string, error)
	PrepareMessagesForPostMortem(messages []slack.Message, description string) (string, error)
}

type AIRepository struct {
	client *openai.Client
	model  string
}

func NewAIRepository() (*AIRepository, error) {
	if os.Getenv("OPENAI_API_KEY") == "" && os.Getenv("AZURE_OPENAI_KEY") == "" {
		return nil, nil
	}

	var model = "gpt-4"
	if os.Getenv("OPENAI_MODEL") != "" {
		model = os.Getenv("OPENAI_MODEL")
	}
	client, err := newOpenAIClient()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OpenAI client: %w", err)
	}
	return &AIRepository{
		client: client,
		model:  model,
	}, nil
}

func newOpenAIClient() (*openai.Client, error) {
	if os.Getenv("AZURE_OPENAI_ENDPOINT") != "" {
		return newAzureClient()
	}

	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is not set")
	}
	options := []option.RequestOption{
		option.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
	}

	c := openai.NewClient(options...)
	return &c, nil
}

func newAzureClient() (*openai.Client, error) {
	key := os.Getenv("AZURE_OPENAI_KEY")
	if key == "" {
		return nil, fmt.Errorf("AZURE_OPENAI_KEY is not set")
	}
	var azureOpenAIEndpoint = os.Getenv("AZURE_OPENAI_ENDPOINT")

	var azureOpenAIAPIVersion = "2025-01-01-preview"

	if os.Getenv("AZURE_OPENAI_API_VERSION") != "" {
		azureOpenAIAPIVersion = os.Getenv("AZURE_OPENAI_API_VERSION")
	}

	c := openai.NewClient(
		azure.WithEndpoint(azureOpenAIEndpoint, azureOpenAIAPIVersion),
		azure.WithAPIKey(key),
	)
	return &c, nil
}

func (h *AIRepository) Summarize(description, slackMessages string) (string, error) {
	prompt := fmt.Sprintf(`## 依頼内容
インシデント対応に関する事象のサマリを作成してください。
あなたには人間が考えた事象の概要と、Slackのメッセージが与えられます。

## フォーマットの指定：
500文字以内で、事象の概要を記載してください。
あなたから受け取った文章はそのまま私の定義したテンプレートに埋め込むので構造化文字列ではなく、概要だけを返却してください。

## 人間が考えた事象の概要
%s

## 関連するSlackのメッセージ
%s`, description, slackMessages)

	return h.callOpenAIWithRetry(prompt)
}

func (h *AIRepository) SummarizeProgress(description, slackMessages string) (string, error) {
	prompt := fmt.Sprintf(`## 依頼内容
これまでのインシデント対応状況をまとめた進捗サマリを作成してください。
あなたには人間が考えた事象の概要と、Slackのメッセージが与えられます。

## フォーマットの指定：
Slack投稿用として3000文字以内で、関係者向けの報告として適切な内容で出力してください。
以下の構成で記載してください：

### 📊 インシデント概要
- 事象の簡潔な説明
- 影響範囲とレベル

### 🔄 現在の状況
- 復旧済み/対応中/調査中のステータス
- 最新の対応状況

### ✅ 実施済み対応
- これまでに実施した対応内容
- 効果があった対策

### 🎯 次のアクション
- 予定されている対応

### 📢 関係者への情報
- 重要な注意点

## 重要な指示：
- **提供されたSlackメッセージに明確に記載されていない情報は推測せず、「詳細不明」「情報不足」「確認中」などと記載してください**
- 不確実な情報や推測に基づく内容は含めないでください
- メッセージに具体的な記載がない場合は「メッセージから詳細を確認できませんでした」と正直に記載してください
- あなたから受け取った文章はそのまま私の定義したテンプレートに埋め込むので、上記の構造化されたフォーマットで返却してください

## 人間が考えた事象の概要
%s

## 関連するSlackのメッセージ
%s`, description, slackMessages)

	return h.callOpenAIWithRetry(prompt)
}

// 高度な進捗サマリ生成（トークン制限対応・分割処理対応）
func (h *AIRepository) SummarizeProgressAdvanced(description string, messages []slack.Message, previousSummary string) (string, error) {
	// トークン計算機を初期化
	tokenCalc, err := NewTokenCalculator()
	if err != nil {
		// フォールバック: 従来の方式
		return h.SummarizeProgress(description, h.formatMessagesSimple(messages))
	}

	// ベースプロンプトを構築
	var basePrompt string
	if previousSummary != "" {
		basePrompt = h.createIncrementalPrompt(description, previousSummary)
	} else {
		basePrompt = h.createInitialPrompt(description)
	}

	// トークン数をチェック
	totalTokens := tokenCalc.CountMessagesTokens(messages, basePrompt)

	// トークン制限内の場合は一度に処理
	if totalTokens <= GetMaxTokens() {
		return h.processSingleChunk(basePrompt, messages, tokenCalc)
	}

	// トークン制限を超える場合は分割処理
	return h.processMultipleChunks(basePrompt, messages, tokenCalc)
}

// 増分更新用プロンプト作成
func (h *AIRepository) createIncrementalPrompt(description, previousSummary string) string {
	return fmt.Sprintf(`## 依頼内容
インシデント対応の進捗サマリを更新してください。
前回のサマリに新しい情報を統合して、最新の状況を反映したサマリを作成してください。

## フォーマット指定
Slack投稿用として3000文字以内で、以下の構成で記載してください：

### 📊 インシデント概要
- 事象の簡潔な説明
- 影響範囲とレベル

### 🔄 現在の状況
- 復旧済み/対応中/調査中のステータス
- 最新の対応状況

### ✅ 実施済み対応
- これまでに実施した対応内容
- 効果があった対策

### 🎯 次のアクション
- 予定されている対応
- 今後の方針

### 📢 関係者への情報
- 重要な注意点
- 協力依頼事項

## 重要な指示：
- **提供されたSlackメッセージに明確に記載されていない情報は推測せず、「詳細不明」「情報不足」「確認中」などと記載してください**
- 不確実な情報や推測に基づく内容は含めないでください
- メッセージに具体的な記載がない場合は「メッセージから詳細を確認できませんでした」と正直に記載してください

## インシデント概要
%s

## 前回のサマリ
%s

## 新しい情報（Slackメッセージ）`, description, previousSummary)
}

// 初回作成用プロンプト作成
func (h *AIRepository) createInitialPrompt(description string) string {
	return fmt.Sprintf(`## 依頼内容
これまでのインシデント対応状況をまとめた進捗サマリを作成してください。
あなたには人間が考えた事象の概要と、Slackのメッセージが与えられます。

## フォーマット指定
Slack投稿用として3000文字以内で、関係者向けの報告として適切な内容で出力してください。
以下の構成で記載してください：

### 📊 インシデント概要
- 事象の簡潔な説明
- 影響範囲とレベル

### 🔄 現在の状況
- 復旧済み/対応中/調査中のステータス
- 最新の対応状況

### ✅ 実施済み対応
- これまでに実施した対応内容
- 効果があった対策

### 🎯 次のアクション
- 予定されている対応
- 今後の方針

### 📢 関係者への情報
- 重要な注意点
- 協力依頼事項

## 重要な指示：
- **提供されたSlackメッセージに明確に記載されていない情報は推測せず、「詳細不明」「情報不足」「確認中」などと記載してください**
- 不確実な情報や推測に基づく内容は含めないでください
- メッセージに具体的な記載がない場合は「メッセージから詳細を確認できませんでした」と正直に記載してください
- あなたから受け取った文章はそのまま私の定義したテンプレートに埋め込むので、上記の構造化されたフォーマットで返却してください

## 人間が考えた事象の概要
%s

## 関連するSlackのメッセージ`, description)
}

// 単一チャンクでの処理
func (h *AIRepository) processSingleChunk(basePrompt string, messages []slack.Message, tokenCalc *TokenCalculator) (string, error) {
	var messageText strings.Builder
	for _, msg := range messages {
		messageText.WriteString(tokenCalc.FormatMessage(msg))
		messageText.WriteString("\n")
	}

	fullPrompt := basePrompt + "\n" + messageText.String()
	return h.callOpenAIWithRetry(fullPrompt)
}

// 複数チャンクでの分割処理
func (h *AIRepository) processMultipleChunks(basePrompt string, messages []slack.Message, tokenCalc *TokenCalculator) (string, error) {
	// メッセージを重要度付きで分割
	chunks := tokenCalc.SplitMessagesWithPriority(messages, basePrompt, GetMaxTokens())

	if len(chunks) == 0 {
		return "", fmt.Errorf("no messages to process")
	}

	if len(chunks) == 1 {
		return h.processSingleChunk(basePrompt, chunks[0], tokenCalc)
	}

	// 各チャンクで部分サマリを作成
	var partialSummaries []string
	for i, chunk := range chunks {
		chunkPrompt := fmt.Sprintf("%s\n\n## 部分 %d/%d のメッセージ", basePrompt, i+1, len(chunks))

		summary, err := h.processSingleChunk(chunkPrompt, chunk, tokenCalc)
		if err != nil {
			return "", fmt.Errorf("failed to process chunk %d: %w", i+1, err)
		}
		partialSummaries = append(partialSummaries, summary)
	}

	// 部分サマリを統合
	mergePrompt := tokenCalc.CreateMergePrompt(partialSummaries)
	return h.callOpenAIWithRetryWithErrorHandling(mergePrompt)
}

// メッセージを簡単な文字列に変換（フォールバック用）
func (h *AIRepository) formatMessagesSimple(messages []slack.Message) string {
	var builder strings.Builder
	for _, msg := range messages {
		builder.WriteString(fmt.Sprintf("%s: %s\n", msg.User, msg.Text))
	}
	return builder.String()
}

// エラーハンドリング強化版のOpenAI呼び出し
func (h *AIRepository) callOpenAIWithRetryWithErrorHandling(prompt string) (string, error) {
	var result string
	err := retry.Retry(3, time.Second*3, func() error {
		resp, err := h.client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(prompt),
			},
			Model: h.model,
		})
		if err != nil {
			// トークン超過エラーの特別処理
			if strings.Contains(err.Error(), "token") || strings.Contains(err.Error(), "length") {
				return fmt.Errorf("token_limit_exceeded: %w", err)
			}
			return err
		}

		if len(resp.Choices) == 0 {
			return fmt.Errorf("no response from OpenAI")
		}

		result = resp.Choices[0].Message.Content
		return nil
	})

	return result, err
}

func (h *AIRepository) GenerateTitle(description, slackMessages string) (string, error) {
	prompt := fmt.Sprintf(`## 依頼内容
インシデント対応に関する事象のタイトルを作成してください。
あなたには、人間が考えた事象の概要と、Slackのメッセージが与えられます。

## フォーマットの指定：
50文字以内で、事象の特徴を捉えたタイトルを作成してください。
あなたから受け取った文章はそのまま私の定義したテンプレートに埋め込むので構造化文字列ではなく、タイトルだけを返却してください。

## 人間が考えた事象の概要
%s

## 関連するSlackのメッセージ
%s`, description, slackMessages)

	return h.callOpenAIWithRetry(prompt)
}

// 共通のリトライ機能付きOpenAI API呼び出し
func (h *AIRepository) callOpenAIWithRetry(prompt string) (string, error) {
	var result string
	err := retry.Retry(3, time.Second*3, func() error {
		resp, err := h.client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(prompt),
			},
			Model: h.model,
		})
		if err != nil {
			return err
		}

		if len(resp.Choices) == 0 {
			return fmt.Errorf("no response from OpenAI")
		}

		result = resp.Choices[0].Message.Content
		return nil
	})

	return result, err
}

// ステータス生成（解決済み/未解決/クローズ）
func (h *AIRepository) GenerateStatus(description, slackMessages string) (string, error) {
	prompt := fmt.Sprintf(`## 依頼内容
インシデント対応の現在のステータスを判定してください。
あなたには、人間が考えた事象の概要と、Slackのメッセージが与えられます。

## フォーマットの指定：
以下の3つの選択肢から最も適切なものを1つ選んで返却してください：
- 未解決
- 解決済み
- クローズ

情報が不十分で判断できない場合は「情報不足のため手動で記入してください」と返却してください。

## 判定基準：
- 未解決：まだ問題が継続している、または対応中の場合
- 解決済み：問題は解決したが、まだ監視や後処理が必要な場合
- クローズ：完全に対応が終了し、問題が完全に解決された場合

## 人間が考えた事象の概要
%s

## 関連するSlackのメッセージ
%s`, description, slackMessages)

	return h.callOpenAIWithRetry(prompt)
}

// 影響分析生成
func (h *AIRepository) GenerateImpact(description, slackMessages string) (string, error) {
	prompt := fmt.Sprintf(`## 依頼内容
インシデントによる影響を分析してください。
あなたには、人間が考えた事象の概要と、Slackのメッセージが与えられます。

## フォーマットの指定：
200文字以内で、以下の観点から影響を記載してください：
- どのサービスや機能に影響があったか
- どの程度のユーザーに影響があったか
- 影響の期間や範囲
- ビジネスへの影響度

情報が不十分で具体的な影響を推論できない場合は「情報不足のため手動で記入してください」と返却してください。
あなたから受け取った文章はそのまま私の定義したテンプレートに埋め込むので構造化文字列ではなく、影響内容だけを返却してください。

## 人間が考えた事象の概要
%s

## 関連するSlackのメッセージ
%s`, description, slackMessages)

	return h.callOpenAIWithRetry(prompt)
}

// 根本原因分析生成
func (h *AIRepository) GenerateRootCause(description, slackMessages string) (string, error) {
	prompt := fmt.Sprintf(`## 依頼内容
インシデントの根本原因を分析してください。
あなたには、人間が考えた事象の概要と、Slackのメッセージが与えられます。

## フォーマットの指定：
300文字以内で、以下の観点から根本原因を記載してください：
- 技術的な原因（コード、設定、インフラ等）
- プロセス上の原因（手順、チェック体制等）
- 外部要因（依存サービス、環境変化等）

根本原因を特定するための十分な情報がない場合や推測が必要な場合は「情報不足のため詳細調査が必要です。手動で記入してください」と返却してください。
あなたから受け取った文章はそのまま私の定義したテンプレートに埋め込むので構造化文字列ではなく、原因分析だけを返却してください。

## 人間が考えた事象の概要
%s

## 関連するSlackのメッセージ
%s`, description, slackMessages)

	return h.callOpenAIWithRetry(prompt)
}

// トリガー分析生成（障害発見の経緯）
func (h *AIRepository) GenerateTrigger(description, slackMessages string) (string, error) {
	prompt := fmt.Sprintf(`## 依頼内容
インシデントがどのように発見されたかを分析してください。
あなたには、人間が考えた事象の概要と、Slackのメッセージが与えられます。

## フォーマットの指定：
150文字以内で、以下の観点からトリガーを記載してください：
- 監視アラートによる発見
- ユーザーからの報告
- 定期チェックでの発見
- 他の作業中の発見

発見の経緯が不明確な場合は「発見経緯が不明のため手動で記入してください」と返却してください。
あなたから受け取った文章はそのまま私の定義したテンプレートに埋め込むので構造化文字列ではなく、発見経緯だけを返却してください。

## 人間が考えた事象の概要
%s

## 関連するSlackのメッセージ
%s`, description, slackMessages)

	return h.callOpenAIWithRetry(prompt)
}

// 解決策生成
func (h *AIRepository) GenerateSolution(description, slackMessages string) (string, error) {
	prompt := fmt.Sprintf(`## 依頼内容
インシデントの解決策を分析してください。
あなたには、人間が考えた事象の概要と、Slackのメッセージが与えられます。

## フォーマットの指定：
300文字以内で、以下の観点から解決策を記載してください：
- 実施した対応手順
- 一時的な対処法
- 根本的な修正内容
- 再発防止策

実施した解決策が明確でない場合や推測が必要な場合は「解決手順が不明のため手動で記入してください」と返却してください。
あなたから受け取った文章はそのまま私の定義したテンプレートに埋め込むので構造化文字列ではなく、解決策の内容だけを返却してください。

## 人間が考えた事象の概要
%s

## 関連するSlackのメッセージ
%s`, description, slackMessages)

	return h.callOpenAIWithRetry(prompt)
}

// アクションアイテム生成
func (h *AIRepository) GenerateActionItems(description, slackMessages string) (string, error) {
	prompt := fmt.Sprintf(`## 依頼内容
インシデント対応後のアクションアイテムを生成してください。
あなたには、人間が考えた事象の概要と、Slackのメッセージが与えられます。

## フォーマットの指定：
以下の形式でアクションアイテムをリスト形式で返却してください：
- 【根本対応】具体的なタスク内容
- 【緩和策】具体的なタスク内容
- 【改善】具体的なタスク内容

各項目は1行で、担当者は含めずタスク内容のみを記載してください。
最大5つまでのアクションアイテムを生成してください。

具体的なアクションアイテムを提案するための情報が不足している場合は「情報不足のため具体的なアクションアイテムを提案できません。手動で記入してください」と返却してください。

## 人間が考えた事象の概要
%s

## 関連するSlackのメッセージ
%s`, description, slackMessages)

	return h.callOpenAIWithRetry(prompt)
}

// 学んだ教訓生成（3つのセクション）
func (h *AIRepository) GenerateLessonsLearned(description, slackMessages string) (string, string, string, error) {
	// うまくいったこと
	goodPrompt := fmt.Sprintf(`## 依頼内容
インシデント対応でうまくいったことを分析してください。
あなたには、人間が考えた事象の概要と、Slackのメッセージが与えられます。

## フォーマットの指定：
200文字以内で、以下の観点からうまくいった点を記載してください：
- 効果的だった対応手順
- 良かったコミュニケーション
- 役立ったツールや仕組み
- チームワークの良い点

具体的にうまくいった点を特定できない場合は「対応中の良かった点が不明のため手動で記入してください」と返却してください。

## 人間が考えた事象の概要
%s

## 関連するSlackのメッセージ
%s`, description, slackMessages)

	// うまくいかなかったこと
	badPrompt := fmt.Sprintf(`## 依頼内容
インシデント対応でうまくいかなかったことを分析してください。
あなたには、人間が考えた事象の概要と、Slackのメッセージが与えられます。

## フォーマットの指定：
200文字以内で、以下の観点から改善が必要な点を記載してください：
- 対応が遅れた原因
- コミュニケーションの課題
- 不足していたツールや情報
- プロセスの問題点

具体的な改善点を特定できない場合は「改善すべき点が不明のため手動で記入してください」と返却してください。

## 人間が考えた事象の概要
%s

## 関連するSlackのメッセージ
%s`, description, slackMessages)

	// 幸運だったこと
	luckyPrompt := fmt.Sprintf(`## 依頼内容
インシデント対応で幸運だったことを分析してください。
あなたには、人間が考えた事象の概要と、Slackのメッセージが与えられます。

## フォーマットの指定：
200文字以内で、以下の観点から幸運だった点を記載してください：
- 偶然うまくいった要素
- 被害が最小限に済んだ理由
- タイミングが良かった点
- 予想外に役立った要素

幸運な要素を特定できない場合は「幸運だった点が不明のため手動で記入してください」と返却してください。

## 人間が考えた事象の概要
%s

## 関連するSlackのメッセージ
%s`, description, slackMessages)

	good, err := h.callOpenAIWithRetry(goodPrompt)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate good lessons: %w", err)
	}

	bad, err := h.callOpenAIWithRetry(badPrompt)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate bad lessons: %w", err)
	}

	lucky, err := h.callOpenAIWithRetry(luckyPrompt)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate lucky lessons: %w", err)
	}

	return good, bad, lucky, nil
}

// タイムライン整形
func (h *AIRepository) FormatTimeline(rawTimeline string) (string, error) {
	prompt := fmt.Sprintf(`## 依頼内容
インシデント対応のタイムラインを整形してください。
生のタイムラインデータが与えられます。

## フォーマットの指定：
以下の形式で整形してください：
- 時刻は「HH:MM」形式で統一
- 重要な出来事のみを抽出
- 時系列順に並び替え
- 冗長な情報は削除
- 1行につき1つの出来事

例：
09:15 サービスAPIが応答停止
09:18 監視アラートを確認
09:25 インシデントチャンネル作成
09:30 原因調査開始

## 生のタイムライン
%s`, rawTimeline)

	return h.callOpenAIWithRetry(prompt)
}

// 残件分析
func (h *AIRepository) AnalyzeRemainingTasks(description, slackMessages string) (string, error) {
	prompt := fmt.Sprintf(`## 依頼内容
インシデント対応の残件を分析してください。
あなたには人間が考えた事象の概要と、Slackのメッセージが与えられます。

## フォーマットの指定：
200文字以内で、以下の観点から残件を記載してください：
- まだ完了していない対応内容
- 今後実施が必要な作業
- 監視や確認が必要な項目
- 対応待ちの課題

**重要**: Slackメッセージから明確に残件が読み取れる場合のみ記載してください。
情報が不十分な場合は「メッセージから残件を確認できませんでした」と返却してください。
推測や仮定に基づく内容は含めないでください。

あなたから受け取った文章はそのまま表示されるので、構造化文字列ではなく、残件の内容だけを返却してください。

## 人間が考えた事象の概要
%s

## 関連するSlackのメッセージ
%s`, description, slackMessages)

	return h.callOpenAIWithRetry(prompt)
}

// ポストモーテム用のメッセージ前処理（トークン制限対応）
func (h *AIRepository) PrepareMessagesForPostMortem(messages []slack.Message, description string) (string, error) {
	tokenCalc, err := NewTokenCalculator()
	if err != nil {
		return h.formatMessagesSimple(messages), nil
	}

	// ポストモーテム用のベースプロンプト（各AI関数で使用される想定トークン数）
	basePromptTokens := 500

	// メッセージをフォーマット
	var formattedMessages strings.Builder
	for _, msg := range messages {
		formattedMessages.WriteString(tokenCalc.FormatMessage(msg))
		formattedMessages.WriteString("\n")
	}

	totalTokens := tokenCalc.CountTokens(formattedMessages.String()) + basePromptTokens
	if totalTokens <= GetMaxTokens() {
		return formattedMessages.String(), nil
	}

	// トークン制限を超える場合は要約処理
	return h.summarizeMessagesForPostMortem(messages, description, tokenCalc)
}

// ポストモーテム用のメッセージ要約
func (h *AIRepository) summarizeMessagesForPostMortem(messages []slack.Message, description string, tokenCalc *TokenCalculator) (string, error) {
	basePrompt := fmt.Sprintf(`## 依頼内容
以下のSlackメッセージを、ポストモーテム作成に必要な情報を保持しながら要約してください。

## フォーマットの指定：
- 時系列順に重要な出来事をまとめてください
- 技術的な詳細（エラーメッセージ、対応内容など）は保持してください
- 各アイテムは「時刻 担当者: 内容」の形式で記載してください
- 最大50項目程度にまとめてください

## インシデント概要
%s

## Slackメッセージ`, description)

	// メッセージを重要度付きで分割
	chunks := tokenCalc.SplitMessagesWithPriority(messages, basePrompt, GetMaxTokens())

	if len(chunks) == 0 {
		return "", fmt.Errorf("no messages to process")
	}

	if len(chunks) == 1 {
		// 1チャンクの場合は直接要約
		return h.summarizeSingleChunk(basePrompt, chunks[0], tokenCalc)
	}

	// 複数チャンクの場合は各チャンクを要約して統合
	var partialSummaries []string
	for i, chunk := range chunks {
		chunkPrompt := fmt.Sprintf("%s\n\n## 部分 %d/%d のメッセージ", basePrompt, i+1, len(chunks))

		summary, err := h.summarizeSingleChunk(chunkPrompt, chunk, tokenCalc)
		if err != nil {
			return "", fmt.Errorf("failed to summarize chunk %d: %w", i+1, err)
		}
		partialSummaries = append(partialSummaries, summary)
	}

	// 部分要約を統合
	return h.mergePostMortemSummaries(partialSummaries)
}

// 単一チャンクの要約
func (h *AIRepository) summarizeSingleChunk(basePrompt string, messages []slack.Message, tokenCalc *TokenCalculator) (string, error) {
	var messageText strings.Builder
	for _, msg := range messages {
		messageText.WriteString(tokenCalc.FormatMessage(msg))
		messageText.WriteString("\n")
	}

	fullPrompt := basePrompt + "\n" + messageText.String()
	return h.callOpenAIWithRetry(fullPrompt)
}

// ポストモーテム用要約の統合
func (h *AIRepository) mergePostMortemSummaries(summaries []string) (string, error) {
	var builder strings.Builder
	builder.WriteString(`## 依頼内容
以下は複数の部分的なインシデントタイムライン要約です。
これらを統合して、1つの完全なタイムラインを作成してください。

## フォーマットの指定：
- 時系列順に整理してください
- 重複を排除してください
- 重要な出来事のみを保持してください
- 「時刻 担当者: 内容」の形式を維持してください

`)

	for i, summary := range summaries {
		builder.WriteString(fmt.Sprintf("## 部分要約 %d\n%s\n\n", i+1, summary))
	}

	return h.callOpenAIWithRetryWithErrorHandling(builder.String())
}
