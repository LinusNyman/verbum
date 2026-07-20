package ingest

import "github.com/linusnyman/vox/internal/store"

// Raw kaikki (wiktextract) record — only the fields vox keeps. Everything else
// (categories, sense-level categories, head_templates, sound audio, …) is
// ignored on decode, which is the bulk of the strip.
type rawEntry struct {
	Word         string     `json:"word"`
	POS          string     `json:"pos"`
	LangCode     string     `json:"lang_code"`
	Etymology    string     `json:"etymology_text"`
	Senses       []rawSense `json:"senses"`
	Sounds       []rawSound `json:"sounds"`
	Forms        []rawForm  `json:"forms"`
	Translations []rawTrans `json:"translations"`
}

type rawSense struct {
	Glosses  []string     `json:"glosses"`
	Tags     []string     `json:"tags"`
	Examples []rawExample `json:"examples"`
	FormOf   []rawFormOf  `json:"form_of"`
}

type rawExample struct {
	Text string `json:"text"`
}

type rawFormOf struct {
	Word string `json:"word"`
}

type rawSound struct {
	IPA string `json:"ipa"`
}

type rawForm struct {
	Form string   `json:"form"`
	Tags []string `json:"tags"`
}

type rawTrans struct {
	Code string `json:"code"`
	Word string `json:"word"`
}

const maxExamples = 2 // STD tier caps examples per sense

// strip converts a raw record into the stored Entry, or nil to skip it.
func strip(r *rawEntry) *store.Entry {
	if r.Word == "" || r.LangCode == "" {
		return nil
	}
	e := &store.Entry{
		Word: r.Word,
		Lang: r.LangCode,
		POS:  r.POS,
		Etym: r.Etymology,
	}

	// IPA from sounds, de-duplicated.
	seenIPA := map[string]bool{}
	for _, s := range r.Sounds {
		if s.IPA != "" && !seenIPA[s.IPA] {
			seenIPA[s.IPA] = true
			e.IPA = append(e.IPA, s.IPA)
		}
	}

	for _, s := range r.Senses {
		if len(s.Glosses) == 0 && len(s.FormOf) == 0 {
			continue
		}
		sense := store.Sense{Glosses: s.Glosses, Tags: s.Tags}
		for _, ex := range s.Examples {
			if ex.Text == "" {
				continue
			}
			sense.Examples = append(sense.Examples, ex.Text)
			if len(sense.Examples) >= maxExamples {
				break
			}
		}
		if len(s.FormOf) > 0 {
			sense.FormOf = s.FormOf[0].Word
		}
		e.Senses = append(e.Senses, sense)
	}

	// Inflected forms (kept — needed when reading; see report §5).
	seenForm := map[string]bool{}
	for _, f := range r.Forms {
		if f.Form == "" || f.Form == r.Word || seenForm[f.Form] {
			continue
		}
		seenForm[f.Form] = true
		e.Forms = append(e.Forms, store.Form{Form: f.Form, Tags: f.Tags})
	}

	// Translations: only English carries them; filter to the 8 target langs.
	if r.LangCode == "en" {
		seenTr := map[string]bool{}
		for _, t := range r.Translations {
			if t.Word == "" || !codeSet[t.Code] {
				continue
			}
			key := t.Code + "\x00" + t.Word
			if seenTr[key] {
				continue
			}
			seenTr[key] = true
			e.Trans = append(e.Trans, store.Trans{Code: t.Code, Word: t.Word})
		}
	}

	if len(e.Senses) == 0 && len(e.Forms) == 0 {
		return nil
	}
	return e
}
