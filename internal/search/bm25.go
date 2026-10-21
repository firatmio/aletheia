// Package search is a tiny in-memory BM25 ranker. For the store sizes this
// tool targets (hundreds to a few thousand short notes) it is fast enough to
// rebuild per query, which keeps the Markdown files the single source of
// truth: there is no index to get out of sync.
package search

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	k1       = 1.2
	b        = 0.75
	stemRune = 6 // tokens longer than this are cut to a 6-rune prefix
)

// Tokenize lower-cases, splits on non-alphanumerics and applies a crude
// prefix "stemmer" that works across inflected languages (English, Turkish).
func Tokenize(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) >= 2 {
			if len(cur) > stemRune {
				cur = cur[:stemRune]
			}
			out = append(out, string(cur))
		}
		cur = cur[:0]
	}
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur = append(cur, r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

// EstimateTokens approximates an LLM token count (about 4 characters each).
func EstimateTokens(s string) int {
	if s == "" {
		return 0
	}
	return (utf8.RuneCountInString(s) + 3) / 4
}

// Doc is a document with pre-weighted terms (repeat a term to boost it).
type Doc struct {
	ID    string
	Terms []string
}

// Index is an immutable BM25 index.
type Index struct {
	docs  []Doc
	tf    []map[string]int
	df    map[string]int
	avgdl float64
}

// NewIndex builds an index over docs.
func NewIndex(docs []Doc) *Index {
	ix := &Index{docs: docs, tf: make([]map[string]int, len(docs)), df: map[string]int{}}
	total := 0
	for i, d := range docs {
		m := map[string]int{}
		for _, t := range d.Terms {
			m[t]++
		}
		ix.tf[i] = m
		for t := range m {
			ix.df[t]++
		}
		total += len(d.Terms)
	}
	if len(docs) > 0 {
		ix.avgdl = float64(total) / float64(len(docs))
	}
	return ix
}

// Scores returns BM25 scores (only documents with a positive score).
func (ix *Index) Scores(query []string) map[string]float64 {
	w := map[string]float64{}
	for _, t := range query {
		w[t] = 1
	}
	return ix.WeightedScores(w)
}

// WeightedScores is Scores with a weight per query term, e.g. how often the
// term occurs in a long prompt.
func (ix *Index) WeightedScores(query map[string]float64) map[string]float64 {
	out := map[string]float64{}
	if len(ix.docs) == 0 || ix.avgdl == 0 {
		return out
	}
	n := float64(len(ix.docs))
	for i, d := range ix.docs {
		dl := float64(len(d.Terms))
		var score float64
		for t, weight := range query {
			f := float64(ix.tf[i][t])
			if f == 0 {
				continue
			}
			df := float64(ix.df[t])
			idf := math.Log(1 + (n-df+0.5)/(df+0.5))
			score += weight * idf * f * (k1 + 1) / (f + k1*(1-b+b*dl/ix.avgdl))
		}
		if score > 0 {
			out[d.ID] = score
		}
	}
	return out
}

// Jaccard returns the set similarity of two token lists.
func Jaccard(a, c []string) float64 {
	if len(a) == 0 || len(c) == 0 {
		return 0
	}
	sa := map[string]bool{}
	for _, t := range a {
		sa[t] = true
	}
	sc := map[string]bool{}
	for _, t := range c {
		sc[t] = true
	}
	inter := 0
	for t := range sa {
		if sc[t] {
			inter++
		}
	}
	union := len(sa) + len(sc) - inter
	return float64(inter) / float64(union)
}
