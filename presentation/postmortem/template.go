package postmortem

import "fmt"

func Render(title, createdAt, author, summary, timeline, channelURL string) string {
	return fmt.Sprintf(`
# タイトル

%s

## 発生日付

%s

## 起票者

%s

## ステータス

例: 未解決、解決済み、クローズ

## 概要

%s

## 影響

例: サービスが断続的にダウンし、最大で１割のユーザーが影響を受けました。

## 主な原因

例: ExamleAPIのバグ、設定ミス

## 障害発生のトリガー

例:監視アラート、ユーザーからの報告

## 解決策

例:切り戻し、データベースの再起動

## アクションアイテム
例:
- 【根本対応】原因となったエンドポイントの修正(担当: Aさん)
- 【緩和策】エラーハンドリングの追加(担当: Bさん)

## 学んだ教訓

### うまくいったこと

### うまくいかなかったこと

### 幸運だったこと

## タイムライン

%s

## 補足情報
- [インシデント対応チャンネル](%s)
`, title, createdAt, author, summary, timeline, channelURL)
}
