package repository

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Songmu/retry"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
	"github.com/openai/openai-go/option"
)

type AIRepositorier interface {
	Summarize(description, slackMessages string) (string, error)
	GenerateTitle(description, slackMessages string) (string, error)
	GenerateStatus(description, slackMessages string) (string, error)
	GenerateImpact(description, slackMessages string) (string, error)
	GenerateRootCause(description, slackMessages string) (string, error)
	GenerateTrigger(description, slackMessages string) (string, error)
	GenerateSolution(description, slackMessages string) (string, error)
	GenerateActionItems(description, slackMessages string) (string, error)
	GenerateLessonsLearned(description, slackMessages string) (string, string, string, error) // うまくいったこと、うまくいかなかったこと、幸運だったこと
	FormatTimeline(rawTimeline string) (string, error)
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
