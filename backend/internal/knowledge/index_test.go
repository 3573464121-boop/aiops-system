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

func TestSearchHybridUsesVectors(t *testing.T) {
	p := filepath.Join(t.TempDir(), "kb.md")
	if err := os.WriteFile(p, []byte("# KB\n## 支付超时\n检查上游服务延迟。\n## 磁盘告警\n清理前审批。"), 0600); err != nil {
		t.Fatal(err)
	}
	idx, err := LoadMarkdown(p)
	if err != nil {
		t.Fatal(err)
	}
	// 3 个分块：文档概述 / 支付超时 / 磁盘告警
	idx.SetVectors([][]float32{{1, 0}, {0, 1}, {0.2, 0.9}})
	if !idx.HasVectors() {
		t.Fatal("vectors should be set")
	}
	// BM25 对无关词无命中
	if len(idx.Search("zzznomatch", 3)) != 0 {
		t.Fatal("bm25 should miss")
	}
	// 混合检索：查询向量最接近第 2 个分块（支付超时），即使 BM25 无命中也应召回
	got := idx.SearchHybrid("zzznomatch", []float32{0, 1}, 2)
	if len(got) == 0 {
		t.Fatal("hybrid should recall via vector when bm25 misses")
	}
	if got[0].Title != "支付超时" {
		t.Fatalf("want 支付超时 first, got %q", got[0].Title)
	}
}
