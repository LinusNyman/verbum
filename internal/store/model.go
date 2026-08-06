package store

import "strings"

// Entry is the stripped, stored form of a Wiktionary entry (STD tier).
// JSON keys are deliberately terse: this struct is zstd-compressed into the
// `data` BLOB across millions of rows, so short keys measurably shrink the DB.
type Entry struct {
	Word   string   `json:"w"`
	Lang   string   `json:"l"` // lang_code: en sv de fr es la el gr
	POS    string   `json:"p,omitempty"`
	IPA    []string `json:"ipa,omitempty"`
	Etym   string   `json:"e,omitempty"`
	Senses []Sense  `json:"s,omitempty"`
	Forms  []Form   `json:"f,omitempty"` // inflected forms of this lemma
	Trans  []Trans  `json:"t,omitempty"` // EN->XX translations (English entries only)
}

// Sense is one numbered definition.
type Sense struct {
	Glosses  []string `json:"g,omitempty"`
	Tags     []string `json:"tg,omitempty"` // the word quirks: archaic, countable, British…
	Examples []string `json:"x,omitempty"`  // capped at 2 (STD tier)
	FormOf   string   `json:"fo,omitempty"` // lemma this entry is an inflected form of
}

// Form is an inflected variant listed on a lemma.
type Form struct {
	Form string   `json:"f"`
	Tags []string `json:"t,omitempty"`
}

// Trans is a single EN->XX translation.
type Trans struct {
	Code string `json:"c"`
	Word string `json:"t"`
}

// GlossText joins every gloss for FTS indexing and reverse search.
func (e *Entry) GlossText() string {
	var b strings.Builder
	for _, s := range e.Senses {
		for _, g := range s.Glosses {
			if b.Len() > 0 {
				b.WriteString(" ; ")
			}
			b.WriteString(g)
		}
	}
	return b.String()
}

// IsFormOf reports whether the entry is purely an inflected-form stub and
// returns the lemma it points at.
func (e *Entry) IsFormOf() (string, bool) {
	for _, s := range e.Senses {
		if s.FormOf != "" {
			return s.FormOf, true
		}
	}
	return "", false
}

// Tags collects the distinct quirk labels across all senses.
func (e *Entry) Tags() []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range e.Senses {
		for _, t := range s.Tags {
			if !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	return out
}
