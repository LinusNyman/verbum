package ingest

// Lang describes one target language and how to reach its kaikki slice.
//
// URL quirk (dict-cli-report.md §1): the directory keeps spaces as %20 but the
// filename drops them — "Ancient Greek" → dir "Ancient%20Greek", file
// "AncientGreek". Guessing one form for both 404s.
type Lang struct {
	Code string // lang_code as it appears in records: en sv de fr es la el grc
	Name string // display name
	Dir  string // URL directory segment (spaces as %20)
	File string // filename segment (spaces removed)
}

// Langs is the full 8-language set (STD tier, ~1.6 GB built).
var Langs = []Lang{
	{"en", "English", "English", "English"},
	{"sv", "Swedish", "Swedish", "Swedish"},
	{"de", "German", "German", "German"},
	{"fr", "French", "French", "French"},
	{"es", "Spanish", "Spanish", "Spanish"},
	{"la", "Latin", "Latin", "Latin"},
	{"el", "Greek", "Greek", "Greek"},
	{"grc", "Ancient Greek", "Ancient%20Greek", "AncientGreek"},
}

// URL builds the kaikki JSONL slice URL for a language.
func (l Lang) URL() string {
	return "https://kaikki.org/dictionary/" + l.Dir +
		"/kaikki.org-dictionary-" + l.File + ".jsonl"
}

// ByCode returns the Lang for a code, or false.
func ByCode(code string) (Lang, bool) {
	for _, l := range Langs {
		if l.Code == code {
			return l, true
		}
	}
	return Lang{}, false
}

// codeSet is every target code — used to filter translation tables.
var codeSet = func() map[string]bool {
	m := map[string]bool{}
	for _, l := range Langs {
		m[l.Code] = true
	}
	return m
}()
