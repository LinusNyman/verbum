package cli

const usage = `verbum — offline Wiktionary dictionary

USAGE
  verbum [flags] WORD            look up a word (terse: sense 1 + labels)
  verbum -r "beat, thrash"       reverse: search definitions for a word
  verbum update [--check] [--lang CODE ...]

FLAGS
  -v            full entry (all senses, examples, etymology, translations)
  -e            etymology only
  -q            quirks only (labels: archaic, countable, British…)
  -t CODE       translate → language CODE (repeatable: -t de -t fr)
  -l CODE       restrict source language (repeatable)
  -k            fuzzy: print close spellings of WORD
  -r            treat the argument as a definition and search glosses
  --json        one JSON object per line (machine output)
  -h, --help    this help
  --version     version

LANGUAGES
  en sv de fr es la el gr

NOTES
  Section flags stack: -e -q shows etymology + quirks, nothing else.
  stdin batch: pipe a word list, one lookup per line.
  Exit codes: 0 hit · 1 no match · 2 error. Honours NO_COLOR and $PAGER.
`
