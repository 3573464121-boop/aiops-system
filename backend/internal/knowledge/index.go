package knowledge

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"aiops-mvp/internal/domain"
)

type Chunk struct {
	Title, Content, Source string
	Terms                  []string
}
type Index struct {
	chunks  []Chunk
	df      map[string]int
	avgLen  float64
	vectors [][]float32 // 可选：与 chunks 一一对应的向量，用于向量/混合检索
}

func LoadMarkdown(path string) (*Index, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	chunks := make([]Chunk, 0)
	title := "文档概述"
	var body []string
	flush := func() {
		text := strings.TrimSpace(strings.Join(body, "\n"))
		if text != "" {
			chunks = append(chunks, Chunk{Title: title, Content: text, Source: fmt.Sprintf("%s#%s", path, title), Terms: tokenize(title + " " + text)})
		}
		body = nil
	}
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 4096), 2<<20)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, "## ") {
			flush()
			title = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			continue
		}
		body = append(body, line)
	}
	flush()
	if err := s.Err(); err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("文档没有可索引内容")
	}
	idx := &Index{chunks: chunks, df: map[string]int{}}
	total := 0
	for _, c := range chunks {
		total += len(c.Terms)
		seen := map[string]bool{}
		for _, t := range c.Terms {
			if !seen[t] {
				idx.df[t]++
				seen[t] = true
			}
		}
	}
	idx.avgLen = float64(total) / float64(len(chunks))
	return idx, nil
}

func (i *Index) Size() int {
	if i == nil {
		return 0
	}
	return len(i.chunks)
}

type ranked struct {
	idx   int
	score float64
}

// bm25 返回按 BM25 得分降序排列的分块下标（仅保留 score>0）。
func (i *Index) bm25(query string) []ranked {
	q := tokenize(query)
	if len(q) == 0 {
		return nil
	}
	n := float64(len(i.chunks))
	const k1 = 1.5
	const b = .75
	out := make([]ranked, 0)
	for ci, c := range i.chunks {
		tf := map[string]int{}
		for _, t := range c.Terms {
			tf[t]++
		}
		score := 0.0
		for _, t := range q {
			f := float64(tf[t])
			if f == 0 {
				continue
			}
			df := float64(i.df[t])
			idf := math.Log(1 + (n-df+.5)/(df+.5))
			score += idf * (f * (k1 + 1)) / (f + k1*(1-b+b*float64(len(c.Terms))/i.avgLen))
		}
		if score > 0 {
			out = append(out, ranked{ci, score})
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].score > out[b].score })
	return out
}

func (i *Index) vectorRank(qvec []float32) []ranked {
	out := make([]ranked, 0, len(i.vectors))
	for ci, v := range i.vectors {
		out = append(out, ranked{ci, cosine(qvec, v)})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].score > out[b].score })
	return out
}

func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for k := range a {
		dot += float64(a[k]) * float64(b[k])
		na += float64(a[k]) * float64(a[k])
		nb += float64(b[k]) * float64(b[k])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// SetVectors 注入与分块一一对应的向量。数量不匹配则忽略（保持 BM25 模式）。
func (i *Index) SetVectors(v [][]float32) {
	if i != nil && len(v) == len(i.chunks) && len(i.chunks) > 0 {
		i.vectors = v
	}
}

func (i *Index) HasVectors() bool {
	return i != nil && len(i.vectors) == len(i.chunks) && len(i.chunks) > 0
}

// ChunkTexts 返回每个分块用于生成向量的文本（标题 + 正文）。
func (i *Index) ChunkTexts() []string {
	if i == nil {
		return nil
	}
	out := make([]string, len(i.chunks))
	for j, c := range i.chunks {
		out[j] = c.Title + " " + c.Content
	}
	return out
}

// Search 关键词（BM25）检索。
func (i *Index) Search(query string, limit int) []domain.Evidence {
	if i == nil || limit <= 0 {
		return []domain.Evidence{}
	}
	return i.toEvidence(i.bm25(query), limit)
}

// SearchHybrid 有查询向量且已注入分块向量时，用 BM25 + 向量双路检索并按 RRF(k=60) 融合；
// 否则退回纯 BM25。这样未配置嵌入模型时行为与之前一致。
func (i *Index) SearchHybrid(query string, qvec []float32, limit int) []domain.Evidence {
	if i == nil || limit <= 0 {
		return []domain.Evidence{}
	}
	if qvec == nil || !i.HasVectors() {
		return i.toEvidence(i.bm25(query), limit)
	}
	const k = 60.0
	rrf := map[int]float64{}
	for rank, r := range i.bm25(query) {
		rrf[r.idx] += 1.0 / (k + float64(rank+1))
	}
	for rank, r := range i.vectorRank(qvec) {
		rrf[r.idx] += 1.0 / (k + float64(rank+1))
	}
	fused := make([]ranked, 0, len(rrf))
	for idx, s := range rrf {
		fused = append(fused, ranked{idx, s})
	}
	sort.Slice(fused, func(a, b int) bool { return fused[a].score > fused[b].score })
	return i.toEvidence(fused, limit)
}

func (i *Index) toEvidence(rs []ranked, limit int) []domain.Evidence {
	if limit > 0 && len(rs) > limit {
		rs = rs[:limit]
	}
	max := 0.0
	if len(rs) > 0 {
		max = rs[0].score
	}
	out := make([]domain.Evidence, 0, len(rs))
	for _, r := range rs {
		score := r.score
		if max > 0 {
			score /= max
		}
		c := i.chunks[r.idx]
		out = append(out, domain.Evidence{Type: "knowledge", Title: c.Title, Content: truncate(c.Content, 700), Score: score, Source: c.Source})
	}
	return out
}

var wordRE = regexp.MustCompile(`[A-Za-z0-9_.:/-]+`)

func tokenize(s string) []string {
	s = strings.ToLower(s)
	out := wordRE.FindAllString(s, -1)
	hans := make([]rune, 0)
	flush := func() {
		for j, r := range hans {
			out = append(out, string(r))
			if j+1 < len(hans) {
				out = append(out, string(hans[j:j+2]))
			}
		}
		hans = nil
	}
	for _, r := range []rune(s) {
		if unicode.Is(unicode.Han, r) {
			hans = append(hans, r)
		} else if len(hans) > 0 {
			flush()
		}
	}
	if len(hans) > 0 {
		flush()
	}
	return out
}
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
