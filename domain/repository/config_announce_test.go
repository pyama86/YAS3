package repository

import "testing"

func TestIsManualGlobalAnnouncement(t *testing.T) {
	if !(&Config{ManualGlobalAnnouncement: true}).IsManualGlobalAnnouncement() {
		t.Fatal("true を設定したら true を返すべき")
	}
	if (&Config{ManualGlobalAnnouncement: false}).IsManualGlobalAnnouncement() {
		t.Fatal("false を設定したら false を返すべき")
	}

	// nil レシーバでも panic せず false を返す（テストで nil Config が渡るため）
	var c *Config
	if c.IsManualGlobalAnnouncement() {
		t.Fatal("nil Config は false を返すべき")
	}
}
