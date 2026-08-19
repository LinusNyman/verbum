package render

import "github.com/linusnyman/verbum/internal/store"

// Readable JSON shape for --json (the stored struct uses terse keys to shrink
// the DB; this remaps to full names for the machine-facing output).
type jsonEntry struct {
	Word         string      `json:"word"`
	Lang         string      `json:"lang"`
	POS          string      `json:"pos,omitempty"`
	IPA          []string    `json:"ipa,omitempty"`
	Etymology    string      `json:"etymology,omitempty"`
	Senses       []jsonSense `json:"senses,omitempty"`
	Forms        []string    `json:"forms,omitempty"`
	Translations []jsonTrans `json:"translations,omitempty"`
}

type jsonSense struct {
	Glosses  []string `json:"glosses,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Examples []string `json:"examples,omitempty"`
	FormOf   string   `json:"form_of,omitempty"`
}

type jsonTrans struct {
	Code string `json:"code"`
	Word string `json:"word"`
}

func toJSON(e *store.Entry) jsonEntry {
	out := jsonEntry{
		Word:      e.Word,
		Lang:      e.Lang,
		POS:       e.POS,
		IPA:       e.IPA,
		Etymology: e.Etym,
	}
	for _, s := range e.Senses {
		out.Senses = append(out.Senses, jsonSense{
			Glosses:  s.Glosses,
			Tags:     s.Tags,
			Examples: s.Examples,
			FormOf:   s.FormOf,
		})
	}
	for _, f := range e.Forms {
		out.Forms = append(out.Forms, f.Form)
	}
	for _, t := range e.Trans {
		out.Translations = append(out.Translations, jsonTrans{Code: t.Code, Word: t.Word})
	}
	return out
}
