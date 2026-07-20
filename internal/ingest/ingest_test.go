package ingest_test

import (
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/linusnyman/vox/internal/fuzzy"
	"github.com/linusnyman/vox/internal/ingest"
	"github.com/linusnyman/vox/internal/store"
)

// kaikki-shaped fixtures, one JSON object per line, per language slice.
var fixtures = map[string]string{
	"en": `{"word":"pattern","pos":"noun","lang_code":"en","etymology_text":"From Middle English patron, from Old French patron.","sounds":[{"ipa":"/ˈpæt.ən/"},{"ipa":"/ˈpæt.ən/"}],"senses":[{"glosses":["A design, motif or decorative arrangement."],"tags":["countable"],"examples":[{"text":"the wallpaper's floral pattern"},{"text":"x"},{"text":"y"}]},{"glosses":["A model to be copied."],"tags":["countable","uncountable"]}],"forms":[{"form":"patterns","tags":["plural"]}],"translations":[{"code":"de","word":"Muster"},{"code":"de","word":"Vorlage"},{"code":"fr","word":"motif"},{"code":"sv","word":"mönster"},{"code":"zz","word":"IGNORE"}]}`,
	"la": `{"word":"circinus","pos":"noun","lang_code":"la","senses":[{"glosses":["a pair of compasses"]}]}
{"word":"circino","pos":"verb","lang_code":"la","senses":[{"glosses":["dative/ablative singular of circinus"],"form_of":[{"word":"circinus"}],"tags":["form-of"]}]}`,
	"el": `{"word":"κοπανίζω","pos":"verb","lang_code":"el","senses":[{"glosses":["to beat, thrash"]}]}`,
	"sv": `{"word":"mönster","pos":"noun","lang_code":"sv","senses":[{"glosses":["pattern"]}]}`,
}

func fakeFetcher(t *testing.T) func(string) (io.ReadCloser, error) {
	byURL := map[string]string{}
	for _, code := range []string{"en", "la", "el", "sv"} {
		l, _ := ingest.ByCode(code)
		byURL[l.URL()] = fixtures[code]
	}
	return func(url string) (io.ReadCloser, error) {
		body, ok := byURL[url]
		if !ok {
			t.Fatalf("unexpected fetch URL: %s", url)
		}
		return io.NopCloser(strings.NewReader(body)), nil
	}
}

func buildTestDB(t *testing.T) *store.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "vox.db")
	err := ingest.Run(dbPath, ingest.Options{
		Only:    []string{"en", "la", "el", "sv"},
		Fetcher: fakeFetcher(t),
	})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestExactLookup(t *testing.T) {
	db := buildTestDB(t)
	got, err := db.Lookup("pattern", nil)
	if err != nil || len(got) != 1 {
		t.Fatalf("lookup pattern: got %d entries, err %v", len(got), err)
	}
	e := got[0]
	if e.Lang != "en" || len(e.Senses) != 2 {
		t.Fatalf("unexpected entry: %+v", e)
	}
	if len(e.Senses[0].Examples) != 2 {
		t.Errorf("examples not capped at 2: got %d", len(e.Senses[0].Examples))
	}
	if len(e.IPA) != 1 {
		t.Errorf("IPA not de-duplicated: %v", e.IPA)
	}
}

func TestCaseAndDiacriticFold(t *testing.T) {
	db := buildTestDB(t)
	if got, _ := db.Lookup("PATTERN", nil); len(got) != 1 {
		t.Errorf("case-insensitive lookup failed: %d", len(got))
	}
	// Unaccented query must find the accented Greek headword.
	if got, _ := db.Lookup("κοπανιζω", nil); len(got) != 1 {
		t.Errorf("diacritic-fold lookup failed: %d", len(got))
	}
}

func TestTranslationsFilteredToTargets(t *testing.T) {
	db := buildTestDB(t)
	got, _ := db.Lookup("pattern", nil)
	codes := map[string]bool{}
	for _, tr := range got[0].Trans {
		codes[tr.Code] = true
	}
	if codes["zz"] {
		t.Error("non-target translation code zz was not stripped")
	}
	if !codes["de"] || !codes["sv"] || !codes["fr"] {
		t.Errorf("expected de/sv/fr translations, got %v", codes)
	}
}

func TestFormOfHasLemma(t *testing.T) {
	db := buildTestDB(t)
	got, _ := db.Lookup("circino", nil)
	if len(got) != 1 {
		t.Fatalf("circino not found")
	}
	lemma, ok := got[0].IsFormOf()
	if !ok || lemma != "circinus" {
		t.Fatalf("form-of not detected: %q %v", lemma, ok)
	}
	if lem, _ := db.Lookup(lemma, []string{"la"}); len(lem) != 1 {
		t.Errorf("lemma circinus not resolvable")
	}
}

func TestReverseGlossSearch(t *testing.T) {
	db := buildTestDB(t)
	got, err := db.Reverse("beat, thrash", nil, 20)
	if err != nil {
		t.Fatalf("reverse: %v", err)
	}
	found := false
	for _, e := range got {
		if e.Word == "κοπανίζω" {
			found = true
		}
	}
	if !found {
		t.Errorf("reverse search 'beat, thrash' did not surface κοπανίζω (got %d)", len(got))
	}
}

func TestFuzzySuggests(t *testing.T) {
	db := buildTestDB(t)
	fold := store.Fold("patern")
	cands, _ := db.FuzzyCandidates(fold, nil, 20000)
	fc := make([]fuzzy.Candidate, len(cands))
	for i, c := range cands {
		fc[i] = fuzzy.Candidate{Word: c.Word, Fold: c.Fold}
	}
	matches := fuzzy.Rank(fold, fc, 7)
	if len(matches) == 0 || matches[0].Word != "pattern" {
		t.Errorf("fuzzy 'patern' should suggest 'pattern', got %+v", matches)
	}
}

func TestLangRestriction(t *testing.T) {
	db := buildTestDB(t)
	if got, _ := db.Lookup("pattern", []string{"sv"}); len(got) != 0 {
		t.Errorf("lang filter sv should exclude English 'pattern', got %d", len(got))
	}
}
