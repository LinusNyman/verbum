package store

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Fold normalises a headword for accent- and case-insensitive exact lookup:
// lowercase, then strip combining marks (NFD → drop Mn → NFC). This is the
// Go-side twin of the FTS `remove_diacritics 2` tokenizer, so `verbum κοπανιζω`
// (unaccented) resolves to the stored `κοπανίζω`.
func Fold(s string) string {
	s = strings.ToLower(s)
	d := norm.NFD.String(s)
	var b strings.Builder
	b.Grow(len(d))
	for _, r := range d {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return norm.NFC.String(b.String())
}
