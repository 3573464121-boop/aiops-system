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
	chunks []Chunk
	df     map[string]int
	avgLen float64
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
func (i *Index) Search(query string, limit int) []domain.Evidence {
	if i == nil || limit <= 0 {
		return []domain.Evidence{}
	}
	q := tokenize(query)
	if len(q) == 0 {
		return []domain.Evidence{}
	}
	type hit struct {
		c     Chunk
		score float64
	}
	hits := make([]hit, 0)
	n := float64(len(i.chunks))
	const k1 = 1.5
	const b = .75
	for _, c := range i.chunks {
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
			hits = append(hits, hit{c, score})
		}
	}
	sort.Slice(hits, func(a, b int) bool { return hits[a].score > hits[b].score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]domain.Evidence, 0, len(hits))
	max := 0.0
	if len(hits) > 0 {
		max = hits[0].score
	}
	for _, h := range hits {
		score := h.score
		if max > 0 {
			score /= max
		}
		out = append(out, domain.Evidence{Type: "knowledge", Title: h.c.Title, Content: truncate(h.c.Content, 700), Score: score, Source: h.c.Source})
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
