package blocks

import (
	"strings"
	"testing"

	"github.com/pyama86/YAS3/domain/entity"
	"github.com/slack-go/slack"
)

func TestInChannelOptions_ManualToggle(t *testing.T) {
	off := InChannelOptions(false)
	for _, o := range off {
		if o.Value == "announce_global" {
			t.Fatalf("announce_global は無効時に含めてはいけない")
		}
	}

	on := InChannelOptions(true)
	found := false
	for _, o := range on {
		if o.Value == "announce_global" {
			found = true
		}
	}
	if !found {
		t.Fatalf("announce_global は有効時に含めるべき")
	}
	if len(on) != len(off)+1 {
		t.Fatalf("有効時はオプションが1つ多いはず: off=%d on=%d", len(off), len(on))
	}
}

func TestGlobalAnnounceForm(t *testing.T) {
	b := GlobalAnnounceForm("初期事象", "2026-06-09 14:30", "初期影響")
	if len(b.BlockSet) != 3 {
		t.Fatalf("入力ブロックは3つのはず: got %d", len(b.BlockSet))
	}

	wantIDs := []string{"summary_block", "occurred_block", "impact_block"}
	for i, blk := range b.BlockSet {
		ib, ok := blk.(*slack.InputBlock)
		if !ok {
			t.Fatalf("block %d は InputBlock ではない", i)
		}
		if ib.BlockID != wantIDs[i] {
			t.Fatalf("block %d の BlockID = %s, want %s", i, ib.BlockID, wantIDs[i])
		}
		el, ok := ib.Element.(*slack.PlainTextInputBlockElement)
		if !ok {
			t.Fatalf("block %d の Element が PlainTextInput ではない", i)
		}
		// 文字数制限が設定されていること
		if el.MaxLength <= 0 {
			t.Fatalf("block %d に MaxLength が未設定", i)
		}
	}

	// AIが生成した初期値が埋め込まれていること
	summary := b.BlockSet[0].(*slack.InputBlock).Element.(*slack.PlainTextInputBlockElement)
	if summary.InitialValue != "初期事象" {
		t.Fatalf("発生事象の初期値が反映されていない: %q", summary.InitialValue)
	}
}

func TestGlobalAnnouncement_ImpactOmitted(t *testing.T) {
	svc := &entity.Service{ID: 1, Name: "svc"}

	withImpact := GlobalAnnouncement("概要", "2026-06-09 14:30", "影響あり", "C1", svc)
	if !containsSectionText(withImpact, "影響範囲") {
		t.Fatalf("影響範囲があるときは影響範囲セクションを含めるべき")
	}

	without := GlobalAnnouncement("概要", "2026-06-09 14:30", "", "C1", svc)
	if containsSectionText(without, "影響範囲") {
		t.Fatalf("影響範囲が空のときは影響範囲セクションを省くべき")
	}
	if !containsSectionText(without, "概要") {
		t.Fatalf("発生事象は常に含めるべき")
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("hello", 10); got != "hello" {
		t.Fatalf("上限以内の文字列は変更しない: got %q", got)
	}
	got := truncateRunes("あいうえお", 3)
	if r := []rune(got); len(r) != 3 {
		t.Fatalf("3ルーンに収めるべき: got %d (%q)", len(r), got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("省略記号で終わるべき: got %q", got)
	}
}

func containsSectionText(blocks []slack.Block, sub string) bool {
	for _, b := range blocks {
		sec, ok := b.(*slack.SectionBlock)
		if !ok {
			continue
		}
		if sec.Text != nil && strings.Contains(sec.Text.Text, sub) {
			return true
		}
		for _, f := range sec.Fields {
			if strings.Contains(f.Text, sub) {
				return true
			}
		}
	}
	return false
}
