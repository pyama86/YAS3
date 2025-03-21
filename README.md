# YAS3 (Yet Another sssbot)

**yas3** は Slack をベースとしたインシデントマネジメントボットです。
「Yet Another sssbot（インシデント対応支援ボット）」として、Slackチャンネルでインシデントの受付から報告、復旧、ポストモーテム作成までを支援します。

sssbotとは[@hiboma](https://github.com/hiboma) が開発しているインシデント支援ボットです。詳細は下記を参照してください。

<iframe class="speakerdeck-iframe" frameborder="0" src="https://speakerdeck.com/player/78f2fd4e8069455fb43e8902fac6b0f4" title="インシデントレスポンスを自動化で支援する  Slack Bot で人機一体なセキュリティ対策を実現する  " allowfullscreen="true" style="border: 0px; background: padding-box padding-box rgba(0, 0, 0, 0.1); margin: 0px; padding: 0px; border-radius: 6px; box-shadow: rgba(0, 0, 0, 0.2) 0px 5px 40px; width: 100%; height: auto; aspect-ratio: 560 / 315;" data-ratio="1.7777777777777777"></iframe>

## Features

- Slack App Mention からインシデントチャンネルを作成
- インシデントの緊急度/レベル管理
- インシデント対応メンバーのアサイン、タイムキーパー通知（15分ごと）
- インシデントの復旧宣言と通知
- Slackのピン留めメッセージや履歴をもとにポストモーテムをAI生成
- Airtable / DynamoDB / OpenAI 連携

### 1. 必要な環境変数を設定

`.env`ファイル または環境変数で以下をセットしてください:

```bash
SLACK_BOT_TOKEN=xoxb-xxxxxxx
SLACK_APP_TOKEN=xapp-xxxxxxx
# (Optional) OpenAI を使う場合
OPENAI_API_KEY=sk-xxxxxx
# または Azure OpenAI を使う場合
AZURE_OPENAI_KEY=xxxxxx
AZURE_OPENAI_ENDPOINT=https://xxx.openai.azure.com/
AZURE_OPENAI_API_VERSION=2025-01-01-preview
```

### 2. 設定ファイルを作成
デフォルトでは $HOME/yas3.toml を読み込みます。

```toml
[[services]]
id = 1
name = "APIサービス"
incident_team_members = ["team-api"]

[[incident_levels]]
level = 1
description = "一部ユーザーに影響"
```

### Slack 上での利用例

- @yas3 とメンション → インシデントチャンネル作成
- インシデント内でメンションのボットメニューから各種操作が可能です
- ポストモーテム作成 ボタン → AI による自動生成 & Slack へアップロード

## License
- MIT License
