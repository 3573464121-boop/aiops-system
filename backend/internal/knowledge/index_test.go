package knowledge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndSearch(t *testing.T) {
	p := filepath.Join(t.TempDir(), "kb.md")
	if err := os.WriteFile(p, []byte("# KB\n## 支付超时\n检查上游服务延迟和连接池等待。\n## 磁盘告警\n清理磁盘前必须审批。"), 0600); err != nil {
		t.Fatal(err)
	}
	idx, err := LoadMarkdown(p)
	if err != nil {
		t.Fatal(err)
	}
	if idx.Size() != 3 {
		t.Fatalf("want preface plus 2 chunks, got %d", idx.Size())
	}
	got := idx.Search("支付服务超时", 3)
	if len(got) == 0 || got[0].Title != "支付超时" {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestSearchEmpty(t *testing.T) {
	var idx *Index
	if got := idx.Search("x", 5); len(got) != 0 {
		t.Fatal("nil index should return empty slice")
	}
}
