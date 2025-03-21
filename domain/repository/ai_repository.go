package repository

import (
	"context"
	"fmt"
	"os"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
	"github.com/openai/openai-go/option"
)

type AIRepositorier interface {
	Summarize(description, slackMessages string) (string, error)
	GenerateTitle(description, slackMessages string) (string, error)
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

	response, err := h.client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
		Model: h.model,
	})

	if err != nil {
		return "", fmt.Errorf("failed to call OpenAI API: %w", err)
	}
	return response.Choices[0].Message.Content, nil
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

	response, err := h.client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
		Model: h.model,
	})
	if err != nil {
		return "", fmt.Errorf("failed to call OpenAI API: %w", err)
	}
	return response.Choices[0].Message.Content, nil
}
