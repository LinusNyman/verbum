// Package render formats entries as man-style ANSI text and handles paging.
package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/linusnyman/verbum/internal/store"
)

// Options controls what a lookup prints.
type Options struct {
	Verbose    bool     // -v : full entry
	Etym       bool     // -e : etymology section
	Quirks     bool     // -q : quirk/tag labels
	Color      bool     // ANSI on/off (TTY + NO_COLOR)
	TransLangs []string // -t : translation-focused output for these codes
}

// ANSI helpers ------------------------------------------------------------

const (
	bold = "\x1b[1m"
	dim  = "\x1b[2m"
	ital = "\x1b[3m"
	rst  = "\x1b[0m"
)

func (o Options) c(code, s string) string {
	if !o.Color {
		return s
	}
	return code + s + rst
}

// Entries renders one or more entries (homographs) into a single string.
func Entries(entries []*store.Entry, opts Options) string {
	var b strings.Builder
	for i, e := range entries {
		if i > 0 {
			b.WriteString("\n")
		}
		renderOne(&b, e, opts)
	}
	return b.String()
}

func renderOne(b *strings.Builder, e *store.Entry, o Options) {
	// Translation-focused output (-t): compact, one line per target language.
	if len(o.TransLangs) > 0 && !o.Verbose && !o.Etym && !o.Quirks {
		renderTrans(b, e, o)
		return
	}

	header(b, e, o)

	switch {
	case o.Etym || o.Quirks:
		if o.Quirks {
			renderQuirks(b, e, o)
		}
		if o.Etym {
			renderEtym(b, e, o)
		}
	case o.Verbose:
		renderSenses(b, e, o, 0)
		renderEtym(b, e, o)
		renderForms(b, e, o)
		renderAllTrans(b, e, o)
	default: // terse
		renderSenses(b, e, o, 1)
	}
}

func header(b *strings.Builder, e *store.Entry, o Options) {
	fmt.Fprintf(b, "%s", o.c(bold, e.Word))
	if e.POS != "" {
		fmt.Fprintf(b, "  %s", o.c(dim, e.POS))
	}
	if len(e.IPA) > 0 {
		fmt.Fprintf(b, "  %s", o.c(dim, strings.Join(dedupe(e.IPA), ", ")))
	}
	b.WriteString("\n")
	if lemma, ok := e.IsFormOf(); ok {
		fmt.Fprintf(b, "  %s %s\n", o.c(dim, "inflected form of"), o.c(bold, lemma))
	}
}

// renderSenses prints numbered definitions. limit==0 means all.
func renderSenses(b *strings.Builder, e *store.Entry, o Options, limit int) {
	n := 0
	for _, s := range e.Senses {
		if len(s.Glosses) == 0 {
			continue
		}
		n++
		if limit > 0 && n > limit {
			break
		}
		if tags := visible(s.Tags); len(tags) > 0 {
			fmt.Fprintf(b, "  %s\n", o.c(ital, "("+strings.Join(tags, ", ")+")"))
		}
		fmt.Fprintf(b, "  %d. %s\n", n, strings.Join(s.Glosses, "; "))
		if o.Verbose {
			for _, ex := range s.Examples {
				fmt.Fprintf(b, "       %s\n", o.c(dim, "\""+ex+"\""))
			}
		}
	}
}

func renderQuirks(b *strings.Builder, e *store.Entry, o Options) {
	tags := visible(e.Tags())
	if len(tags) == 0 {
		fmt.Fprintf(b, "  %s\n", o.c(dim, "(no labels)"))
		return
	}
	fmt.Fprintf(b, "  %s\n", strings.Join(tags, ", "))
}

func renderEtym(b *strings.Builder, e *store.Entry, o Options) {
	if e.Etym == "" {
		if o.Etym {
			fmt.Fprintf(b, "\n%s\n  %s\n", o.c(bold, "ETYMOLOGY"), o.c(dim, "(none recorded)"))
		}
		return
	}
	fmt.Fprintf(b, "\n%s\n", o.c(bold, "ETYMOLOGY"))
	fmt.Fprintf(b, "  %s\n", e.Etym)
}

func renderForms(b *strings.Builder, e *store.Entry, o Options) {
	if len(e.Forms) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s\n", o.c(bold, "FORMS"))
	var parts []string
	for _, f := range e.Forms {
		if f.Form == "" {
			continue
		}
		parts = append(parts, f.Form)
		if len(parts) >= 24 {
			break
		}
	}
	fmt.Fprintf(b, "  %s\n", strings.Join(dedupe(parts), ", "))
}

func renderAllTrans(b *strings.Builder, e *store.Entry, o Options) {
	if len(e.Trans) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s\n", o.c(bold, "TRANSLATIONS"))
	for code, words := range groupTrans(e.Trans, nil) {
		fmt.Fprintf(b, "  %s  %s\n", o.c(dim, code), strings.Join(words, ", "))
	}
}

func renderTrans(b *strings.Builder, e *store.Entry, o Options) {
	fmt.Fprintf(b, "%s\n", o.c(bold, e.Word))
	grouped := groupTrans(e.Trans, o.TransLangs)
	if len(grouped) == 0 {
		fmt.Fprintf(b, "  %s\n", o.c(dim, "(no translations for "+strings.Join(o.TransLangs, ", ")+")"))
		return
	}
	for _, code := range o.TransLangs {
		if words, ok := grouped[code]; ok {
			fmt.Fprintf(b, "  %s  %s\n", o.c(dim, code), strings.Join(words, ", "))
		}
	}
}

// JSONLines renders entries as one compact JSON object per line (jq-friendly).
func JSONLines(entries []*store.Entry) (string, error) {
	var b strings.Builder
	for _, e := range entries {
		obj := toJSON(e)
		raw, err := json.Marshal(obj)
		if err != nil {
			return "", err
		}
		b.Write(raw)
		b.WriteString("\n")
	}
	return b.String(), nil
}

// ---- helpers ------------------------------------------------------------

func groupTrans(ts []store.Trans, only []string) map[string][]string {
	want := map[string]bool{}
	for _, c := range only {
		want[c] = true
	}
	out := map[string][]string{}
	seen := map[string]bool{}
	for _, t := range ts {
		if len(only) > 0 && !want[t.Code] {
			continue
		}
		key := t.Code + "\x00" + t.Word
		if seen[key] {
			continue
		}
		seen[key] = true
		out[t.Code] = append(out[t.Code], t.Word)
	}
	return out
}

// visible drops structural tags (like "form-of") that aren't usage labels;
// the form-of relationship is already shown on the header line.
func visible(tags []string) []string {
	var out []string
	for _, t := range tags {
		if t == "form-of" {
			continue
		}
		out = append(out, t)
	}
	return out
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
