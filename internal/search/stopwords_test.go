package search

import (
	"reflect"
	"testing"
)

func TestContentTermsDropsFillerInBothLanguages(t *testing.T) {
	got := ContentTerms(Tokenize("How does the stop hook work? Bu hook nasıl çalışıyor, eğer varsa"))
	want := []string{"stop", "hook", "hook", "çalışı", "varsa"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestContentTermsDropsShortNumbersOnly(t *testing.T) {
	got := ContentTerms(Tokenize("#11 merged, see go 1.23 and the 2024-11-05 protocol, port 8080"))
	want := []string{"merged", "see", "go", "2024", "protoc", "port", "8080"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
