package search

import (
	"reflect"
	"testing"
)

func TestTokenizeStemsInflections(t *testing.T) {
	got := Tokenize("Authentication uses JWT tokens, v1.23!")
	want := []string{"authen", "uses", "jwt", "tokens", "v1", "23"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	// Turkish inflections collapse to the same stem.
	a, b := Tokenize("kullanıyor"), Tokenize("kullanır")
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("expected the same stem: %v vs %v", a, b)
	}
}

func TestBM25Ranking(t *testing.T) {
	ix := NewIndex([]Doc{
		{ID: "auth", Terms: Tokenize("login uses jwt auth tokens")},
		{ID: "db", Terms: Tokenize("database migrations run on postgres")},
		{ID: "both", Terms: Tokenize("jwt secrets stored in the database")},
	})
	s := ix.Scores(Tokenize("jwt auth"))
	if s["auth"] <= s["both"] || s["both"] <= 0 {
		t.Fatalf("unexpected ranking: %v", s)
	}
	if _, ok := s["db"]; ok {
		t.Fatalf("irrelevant doc should not score: %v", s)
	}
	if len(NewIndex(nil).Scores(Tokenize("x y"))) != 0 {
		t.Fatal("empty index must score nothing")
	}
}

func TestRepeatedTermsBoost(t *testing.T) {
	ix := NewIndex([]Doc{
		{ID: "boosted", Terms: append(Tokenize("redis cache"), Tokenize("redis cache redis cache")...)},
		{ID: "plain", Terms: Tokenize("redis cache plus other filler words here")},
	})
	s := ix.Scores(Tokenize("redis"))
	if s["boosted"] <= s["plain"] {
		t.Fatalf("expected boost: %v", s)
	}
}

func TestJaccard(t *testing.T) {
	a := Tokenize("the backend uses go and postgres")
	if Jaccard(a, a) != 1 {
		t.Fatal("identical sets should be 1")
	}
	if Jaccard(a, Tokenize("completely unrelated words")) != 0 {
		t.Fatal("disjoint sets should be 0")
	}
	if Jaccard(nil, a) != 0 {
		t.Fatal("empty set should be 0")
	}
}

func TestEstimateTokens(t *testing.T) {
	cases := map[string]int{"": 0, "abcd": 1, "abcde": 2, "ünïcödé!": 2}
	for in, want := range cases {
		if got := EstimateTokens(in); got != want {
			t.Errorf("EstimateTokens(%q) = %d, want %d", in, got, want)
		}
	}
}
