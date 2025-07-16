package postmortem

import "fmt"

func Render(title, createdAt, author, summary, status, impact, rootCause, trigger, solution, actionItems, lessonsGood, lessonsBad, lessonsLucky, timeline, channelURL string) string {
	return fmt.Sprintf(`
# タイトル

%s

## 発生日付

%s

## 起票者

%s

## ステータス

%s

## 概要

%s

## 影響

%s

## 主な原因

%s

## 障害発生のトリガー

%s

## 解決策

%s

## アクションアイテム

%s

## 学んだ教訓

### うまくいったこと

%s

### うまくいかなかったこと

%s

### 幸運だったこと

%s

## タイムライン

%s

## 補足情報
- [インシデント対応チャンネル](%s)
`, title, createdAt, author, status, summary, impact, rootCause, trigger, solution, actionItems, lessonsGood, lessonsBad, lessonsLucky, timeline, channelURL)
}
