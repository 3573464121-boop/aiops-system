package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aiops-mvp/internal/storage"
	"aiops-mvp/internal/tools"
)

func newKnowledgeTestService(t *testing.T) (*Service, context.Context) {
	t.Helper()
	root := t.TempDir()
	seed := filepath.Join(root, "seed.md")
	if err := os.WriteFile(seed, []byte("## 数据库连接池\n连接池耗尽会导致请求超时。"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := New(tools.NewService(nil, nil, nil), nil, storage.NewMemory())
	ctx := WithActor(context.Background(), "USR-admin", "admin", "admin")
	if _, err := s.InitializeKnowledge(ctx, []string{seed}, filepath.Join(root, "managed")); err != nil {
		t.Fatal(err)
	}
	return s, ctx
}

func TestKnowledgeDocumentLifecycle(t *testing.T) {
	s, ctx := newKnowledgeTestService(t)
	doc, err := s.ImportKnowledgeDocument(ctx, "runbook.md", []byte("## Redis 热点\n缓存热点时先检查大键和命中率。"))
	if err != nil {
		t.Fatal(err)
	}
	if !doc.Managed || doc.ChunkCount != 1 {
		t.Fatalf("unexpected imported document: %+v", doc)
	}
	if got := s.Tools.SearchKnowledge("缓存热点 大键", 5); len(got) == 0 || !strings.Contains(got[0].Content, "命中率") {
		t.Fatalf("imported document should be searchable: %+v", got)
	}
	if _, err = s.ImportKnowledgeDocument(ctx, "copy.md", []byte("## Redis 热点\n缓存热点时先检查大键和命中率。")); err == nil {
		t.Fatal("duplicate content should be rejected")
	}
	if _, err = s.ToggleKnowledgeDocument(ctx, doc.ID, false); err != nil {
		t.Fatal(err)
	}
	for _, result := range s.Tools.SearchKnowledge("缓存热点 大键", 5) {
		if strings.Contains(result.Content, "命中率") {
			t.Fatalf("disabled document remained searchable: %+v", result)
		}
	}
	if err = s.DeleteKnowledgeDocument(ctx, doc.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(doc.Path); !os.IsNotExist(err) {
		t.Fatalf("managed file should be removed, stat err=%v", err)
	}
}

func TestKnowledgeUploadValidation(t *testing.T) {
	cases := []struct {
		name, filename string
		content        []byte
	}{
		{name: "path traversal", filename: "../secret.md", content: []byte("valid")},
		{name: "wrong extension", filename: "notes.txt", content: []byte("valid")},
		{name: "empty", filename: "notes.md", content: nil},
		{name: "invalid utf8", filename: "notes.md", content: []byte{0xff, 0xfe}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateKnowledgeUpload(tc.filename, tc.content); err == nil {
				t.Fatal("invalid upload should be rejected")
			}
		})
	}
}

func TestKnowledgeReindexFailureKeepsLiveIndex(t *testing.T) {
	s, _ := newKnowledgeTestService(t)
	docs, err := s.ListKnowledgeDocuments()
	if err != nil || len(docs) != 1 {
		t.Fatalf("unexpected documents: %+v %v", docs, err)
	}
	if err = os.Remove(docs[0].Path); err != nil {
		t.Fatal(err)
	}
	before := s.Tools.KnowledgeSnapshot()
	if _, err = s.ReindexKnowledge(context.Background()); err == nil {
		t.Fatal("reindex should fail when source file is missing")
	}
	if after := s.Tools.KnowledgeSnapshot(); after != before {
		t.Fatal("failed reindex must not replace the live index")
	}
}

func TestKnowledgeMutationBlockedDuringExperiment(t *testing.T) {
	s, ctx := newKnowledgeTestService(t)
	s.beginExperimentBatch()
	defer s.endExperimentBatch()
	if _, err := s.ImportKnowledgeDocument(ctx, "blocked.md", []byte("## 不应导入\n实验运行时保持知识版本固定。")); err == nil {
		t.Fatal("knowledge mutation should be blocked while an experiment is running")
	}
}
