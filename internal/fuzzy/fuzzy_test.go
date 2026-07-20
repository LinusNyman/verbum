package fuzzy

import "testing"

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"patern", "pattern", 1},
		{"kitten", "sitting", 3},
		{"", "abc", 3},
		{"κοπανιζω", "κοπανίζω", 1},
	}
	for _, c := range cases {
		if got := Levenshtein(c.a, c.b); got != c.want {
			t.Errorf("Levenshtein(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestRankOrdersAndFilters(t *testing.T) {
	cands := []Candidate{
		{"pattern", "pattern"},
		{"patterns", "patterns"},
		{"lantern", "lantern"},
		{"paternal", "paternal"},
		{"zzzzzz", "zzzzzz"},
	}
	got := Rank("patern", cands, 3)
	if len(got) == 0 || got[0].Word != "pattern" {
		t.Fatalf("closest should be 'pattern', got %+v", got)
	}
	if len(got) > 3 {
		t.Errorf("n limit not applied: %d", len(got))
	}
	for _, m := range got {
		if m.Word == "zzzzzz" {
			t.Errorf("distant candidate leaked past threshold")
		}
	}
}

func TestRankSkipsExact(t *testing.T) {
	got := Rank("pattern", []Candidate{{"pattern", "pattern"}}, 5)
	if len(got) != 0 {
		t.Errorf("exact match should not be suggested, got %+v", got)
	}
}
