//go:build linux || darwin
// +build linux darwin

package packages

import (
	"fmt"
	"testing"
)

func TestNormalizeQuery(t *testing.T) {
	got := normalizeQuery("Requests-HTML_v2.0")
	want := "requestshtmlv20"
	if got != want {
		t.Fatalf("normalizeQuery() = %q, want %q", got, want)
	}
}

func TestLevenshteinDistance(t *testing.T) {
	if d := levenshteinDistance("kitten", "sitting"); d != 3 {
		t.Fatalf("levenshteinDistance(kitten, sitting) = %d, want 3", d)
	}
	if d := levenshteinDistance("", "abc"); d != 3 {
		t.Fatalf("levenshteinDistance(empty, abc) = %d, want 3", d)
	}
	if d := levenshteinDistance("same", "same"); d != 0 {
		t.Fatalf("levenshteinDistance(same, same) = %d, want 0", d)
	}
}

func TestFuzzyScorePreference(t *testing.T) {
	query := normalizeQuery("req")
	exact := fuzzyScore(query, normalizeQuery("req"))
	prefix := fuzzyScore(query, normalizeQuery("requests"))
	contains := fuzzyScore(query, normalizeQuery("myreqpkg"))
	distant := fuzzyScore(query, normalizeQuery("numpy"))

	if exact <= prefix {
		t.Fatalf("expected exact score (%d) > prefix score (%d)", exact, prefix)
	}
	if prefix <= contains {
		t.Fatalf("expected prefix score (%d) > contains score (%d)", prefix, contains)
	}
	if contains <= distant {
		t.Fatalf("expected contains score (%d) > distant score (%d)", contains, distant)
	}
}

func Example_normalizeQuery() {
	fmt.Println(normalizeQuery("Requests-HTML_v2.0"))
	// Output: requestshtmlv20
}
