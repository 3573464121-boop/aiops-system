package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"aiops-mvp/internal/domain"
	"aiops-mvp/internal/knowledge"
)

const maxKnowledgeDocumentBytes = 2 << 20

func (s *Service) InitializeKnowledge(ctx context.Context, seedPaths []string, managedRoot string) (domain.KnowledgeStatus, error) {
	root, err := filepath.Abs(strings.TrimSpace(managedRoot))
	if err != nil {
		return domain.KnowledgeStatus{}, fmt.Errorf("resolve knowledge directory: %w", err)
	}
	if err = os.MkdirAll(root, 0o755); err != nil {
		return domain.KnowledgeStatus{}, fmt.Errorf("create knowledge directory: %w", err)
	}
	s.knowledgeMu.Lock()
	s.knowledgeRoot = root
	s.knowledgeMu.Unlock()
	s.Tools.OnKnowledgeHits = s.recordKnowledgeHits

	if err = s.seedKnowledgeDocuments(seedPaths); err != nil {
		return domain.KnowledgeStatus{}, err
	}
	s.knowledgeOpMu.Lock()
	defer s.knowledgeOpMu.Unlock()
	return s.reindexKnowledgeLocked(ctx)
}

func (s *Service) seedKnowledgeDocuments(paths []string) error {
	docs, err := s.Repo.ListKnowledgeDocuments()
	if err != nil {
		return err
	}
	byPath := make(map[string]domain.KnowledgeDocument, len(docs))
	byHash := make(map[string]domain.KnowledgeDocument, len(docs))
	for _, doc := range docs {
		byPath[filepath.Clean(doc.Path)] = doc
		byHash[doc.ContentHash] = doc
	}
	for _, rawPath := range paths {
		path, absErr := filepath.Abs(rawPath)
		if absErr != nil {
			return absErr
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read seed knowledge document %s: %w", rawPath, readErr)
		}
		hash := contentHash(content)
		if existing, ok := byPath[filepath.Clean(path)]; ok {
			if existing.ContentHash != hash {
				existing.ContentHash = hash
				existing.Version = shortVersion(hash)
				existing.UpdatedAt = time.Now()
				if err = s.Repo.UpdateKnowledgeDocument(existing); err != nil {
					return err
				}
			}
			continue
		}
		if existing, ok := byHash[hash]; ok {
			if !existing.Managed && filepath.Clean(existing.Path) != filepath.Clean(path) {
				existing.Name = filepath.Base(path)
				existing.Path = path
				existing.UpdatedAt = time.Now()
				if err = s.Repo.UpdateKnowledgeDocument(existing); err != nil {
					return err
				}
			}
			continue
		}
		now := time.Now()
		doc := domain.KnowledgeDocument{
			ID: "KDOC-" + hash[:20], Name: filepath.Base(path), Path: path,
			ContentHash: hash, Version: shortVersion(hash), Enabled: true, Managed: false,
			CreatedBy: "system", CreatedAt: now, UpdatedAt: now,
		}
		if err = s.Repo.CreateKnowledgeDocument(doc); err != nil {
			return err
		}
		byHash[hash] = doc
	}
	return nil
}

func (s *Service) ImportKnowledgeDocument(ctx context.Context, filename string, content []byte) (domain.KnowledgeDocument, error) {
	started := time.Now()
	s.knowledgeOpMu.Lock()
	defer s.knowledgeOpMu.Unlock()

	if err := validateKnowledgeUpload(filename, content); err != nil {
		return domain.KnowledgeDocument{}, err
	}
	if err := s.ensureKnowledgeMutable(); err != nil {
		return domain.KnowledgeDocument{}, err
	}
	docs, err := s.Repo.ListKnowledgeDocuments()
	if err != nil {
		return domain.KnowledgeDocument{}, err
	}
	hash := contentHash(content)
	for _, doc := range docs {
		if doc.ContentHash == hash {
			return domain.KnowledgeDocument{}, fmt.Errorf("相同内容已存在于文档 %s", doc.Name)
		}
	}
	s.knowledgeMu.RLock()
	root := s.knowledgeRoot
	s.knowledgeMu.RUnlock()
	if root == "" {
		return domain.KnowledgeDocument{}, fmt.Errorf("知识库管理目录尚未初始化")
	}

	now := time.Now()
	id := "KDOC-" + hash[:20]
	path := filepath.Join(root, id+".md")
	tmp := path + ".tmp"
	if err = os.WriteFile(tmp, content, 0o600); err != nil {
		return domain.KnowledgeDocument{}, fmt.Errorf("write knowledge document: %w", err)
	}
	if err = os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return domain.KnowledgeDocument{}, fmt.Errorf("publish knowledge document: %w", err)
	}
	_, username, _, _ := actorFromContext(ctx)
	doc := domain.KnowledgeDocument{
		ID: id, Name: filename, Path: path, ContentHash: hash, Version: shortVersion(hash),
		Enabled: true, Managed: true, CreatedBy: username, CreatedAt: now, UpdatedAt: now,
	}
	if err = s.Repo.CreateKnowledgeDocument(doc); err != nil {
		_ = os.Remove(path)
		return domain.KnowledgeDocument{}, err
	}
	if _, err = s.reindexKnowledgeLocked(ctx); err != nil {
		_ = s.Repo.DeleteKnowledgeDocument(doc.ID)
		_ = os.Remove(path)
		return domain.KnowledgeDocument{}, err
	}
	created, err := s.Repo.GetKnowledgeDocument(doc.ID)
	if err == nil {
		s.addAudit(ctx, "knowledge_import", "", "success", time.Since(started).Milliseconds())
	}
	return created, err
}

func (s *Service) ListKnowledgeDocuments() ([]domain.KnowledgeDocument, error) {
	return s.Repo.ListKnowledgeDocuments()
}

func (s *Service) ToggleKnowledgeDocument(ctx context.Context, id string, enabled bool) (domain.KnowledgeDocument, error) {
	started := time.Now()
	s.knowledgeOpMu.Lock()
	defer s.knowledgeOpMu.Unlock()
	if err := s.ensureKnowledgeMutable(); err != nil {
		return domain.KnowledgeDocument{}, err
	}
	doc, err := s.Repo.GetKnowledgeDocument(id)
	if err != nil {
		return domain.KnowledgeDocument{}, err
	}
	previous := doc
	doc.Enabled = enabled
	doc.UpdatedAt = time.Now()
	if err = s.Repo.UpdateKnowledgeDocument(doc); err != nil {
		return domain.KnowledgeDocument{}, err
	}
	if _, err = s.reindexKnowledgeLocked(ctx); err != nil {
		_ = s.Repo.UpdateKnowledgeDocument(previous)
		return domain.KnowledgeDocument{}, err
	}
	updated, err := s.Repo.GetKnowledgeDocument(id)
	if err == nil {
		s.addAudit(ctx, "knowledge_toggle", "", "success", time.Since(started).Milliseconds())
	}
	return updated, err
}

func (s *Service) DeleteKnowledgeDocument(ctx context.Context, id string) error {
	started := time.Now()
	s.knowledgeOpMu.Lock()
	defer s.knowledgeOpMu.Unlock()
	if err := s.ensureKnowledgeMutable(); err != nil {
		return err
	}
	doc, err := s.Repo.GetKnowledgeDocument(id)
	if err != nil {
		return err
	}
	if !doc.Managed {
		return fmt.Errorf("内置知识文档不能删除，可以将其停用")
	}
	if err = ensureWithinRoot(s.knowledgeRoot, doc.Path); err != nil {
		return err
	}
	if err = s.Repo.DeleteKnowledgeDocument(id); err != nil {
		return err
	}
	if _, err = s.reindexKnowledgeLocked(ctx); err != nil {
		_ = s.Repo.CreateKnowledgeDocument(doc)
		return err
	}
	if err = os.Remove(doc.Path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove knowledge document: %w", err)
	}
	s.addAudit(ctx, "knowledge_delete", "", "success", time.Since(started).Milliseconds())
	return nil
}

func (s *Service) ReindexKnowledge(ctx context.Context) (domain.KnowledgeStatus, error) {
	started := time.Now()
	s.knowledgeOpMu.Lock()
	defer s.knowledgeOpMu.Unlock()
	if err := s.ensureKnowledgeMutable(); err != nil {
		return domain.KnowledgeStatus{}, err
	}
	status, err := s.reindexKnowledgeLocked(ctx)
	if err == nil {
		s.addAudit(ctx, "knowledge_reindex", "", "success", time.Since(started).Milliseconds())
	}
	return status, err
}

func (s *Service) reindexKnowledgeLocked(_ context.Context) (domain.KnowledgeStatus, error) {
	docs, err := s.Repo.ListKnowledgeDocuments()
	if err != nil {
		return domain.KnowledgeStatus{}, err
	}
	enabled := make([]domain.KnowledgeDocument, 0, len(docs))
	paths := make([]string, 0, len(docs))
	for _, doc := range docs {
		if doc.Enabled {
			enabled = append(enabled, doc)
			paths = append(paths, doc.Path)
		}
	}
	sort.Strings(paths)
	var index *knowledge.Index
	warning := ""
	if len(paths) > 0 {
		index, err = knowledge.LoadMarkdownFiles(paths)
		if err != nil {
			return domain.KnowledgeStatus{}, fmt.Errorf("build knowledge index: %w", err)
		}
		if s.Tools.EmbedDocuments != nil {
			vectors, embedErr := s.Tools.EmbedDocuments(index.ChunkTexts())
			if embedErr != nil {
				warning = "向量构建失败，当前索引已回退为 BM25"
			} else {
				index.SetVectors(vectors)
			}
		}
	}
	counts := map[string]int{}
	if index != nil {
		counts = index.SourceCounts()
	}
	now := time.Now()
	pathToID := make(map[string]string, len(enabled))
	for _, doc := range docs {
		if doc.Enabled {
			doc.ChunkCount = counts[doc.Path]
			pathToID[filepath.Clean(doc.Path)] = doc.ID
		} else {
			doc.ChunkCount = 0
		}
		doc.UpdatedAt = now
		if err = s.Repo.UpdateKnowledgeDocument(doc); err != nil {
			return domain.KnowledgeStatus{}, err
		}
	}
	s.Tools.SetKnowledge(index)
	status := domain.KnowledgeStatus{
		DocumentCount: len(docs), EnabledCount: len(enabled), ChunkCount: s.Tools.KnowledgeSize(),
		Mode: s.Tools.KnowledgeMode(), Version: knowledgeVersion(enabled), LastIndexedAt: now, Warning: warning,
	}
	s.knowledgeMu.Lock()
	s.knowledgeDocs = pathToID
	s.knowledgeState = status
	s.knowledgeMu.Unlock()
	return status, nil
}

func (s *Service) KnowledgeStatus() domain.KnowledgeStatus {
	s.knowledgeMu.RLock()
	defer s.knowledgeMu.RUnlock()
	return s.knowledgeState
}

func (s *Service) ensureKnowledgeMutable() error {
	s.experimentStateMu.RLock()
	active := s.activeExperimentBatches
	s.experimentStateMu.RUnlock()
	if active > 0 {
		return fmt.Errorf("有 %d 个实验批次正在等待或运行，当前不能修改知识库", active)
	}
	return nil
}

func (s *Service) recordKnowledgeHits(evidence []domain.Evidence) {
	s.knowledgeMu.RLock()
	paths := s.knowledgeDocs
	ids := make([]string, 0, len(evidence))
	seen := map[string]bool{}
	for _, item := range evidence {
		source := item.Source
		if pos := strings.LastIndex(source, "#"); pos >= 0 {
			source = source[:pos]
		}
		if id := paths[filepath.Clean(source)]; id != "" && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	s.knowledgeMu.RUnlock()
	_ = s.Repo.IncrementKnowledgeDocumentHits(ids)
}

func validateKnowledgeUpload(filename string, content []byte) error {
	name := strings.TrimSpace(filename)
	if name == "" || strings.ContainsAny(name, `/\\`) || filepath.Base(name) != name {
		return fmt.Errorf("文档名不合法")
	}
	if !strings.EqualFold(filepath.Ext(name), ".md") {
		return fmt.Errorf("只允许导入 Markdown (.md) 文档")
	}
	if len(content) == 0 {
		return fmt.Errorf("文档内容不能为空")
	}
	if len(content) > maxKnowledgeDocumentBytes {
		return fmt.Errorf("文档不能超过 2 MiB")
	}
	if !utf8.Valid(content) {
		return fmt.Errorf("文档必须使用 UTF-8 编码")
	}
	return nil
}

func ensureWithinRoot(root, path string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("拒绝访问知识库管理目录之外的文件")
	}
	return nil
}

func contentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func shortVersion(hash string) string {
	if len(hash) > 12 {
		hash = hash[:12]
	}
	return "sha256:" + hash
}

func knowledgeVersion(docs []domain.KnowledgeDocument) string {
	if len(docs) == 0 {
		return "empty"
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].ID < docs[j].ID })
	h := sha256.New()
	for _, doc := range docs {
		_, _ = fmt.Fprintf(h, "%s:%s\n", doc.ID, doc.ContentHash)
	}
	return "idx:" + hex.EncodeToString(h.Sum(nil))[:16]
}
