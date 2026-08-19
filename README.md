# verbum

An offline, `man`-style terminal dictionary for reading across languages —
English, Swedish, German, French, Spanish, Latin, Greek, and Ancient Greek.
Built for looking up **etymology**, **word quirks** (labels like *archaic*,
*countable*), and **translations**, with fuzzy spell matching when you only
half-remember a word. Unix-composable: quiet, pipe-friendly, exit codes.

```
$ verbum pattern
pattern  noun  /ˈpæt.ən/, /ˈpæt.ɚn/
  (countable)
  1. A design, motif or decorative arrangement.
```

## Install

**Homebrew**

```sh
brew install linusnyman/verbum/verbum
```

**Go**

```sh
go install github.com/linusnyman/verbum/cmd/verbum@latest
```

Or grab a binary from the [releases](https://github.com/linusnyman/verbum/releases).

## Build the dictionary

The binary ships **without data**. Build the local database once (downloads
~1.5 GB of parsed Wiktionary data from [kaikki.org](https://kaikki.org),
streamed and stripped on the fly; final DB ~1.6 GB):

```sh
verbum update                 # all 8 languages
verbum update --lang sv       # or just one, to try it out
verbum update --check         # show installed data age + counts
```

Data lives under `${XDG_DATA_HOME:-~/.local/share}/verbum/verbum.db`.

## Use

```
verbum WORD                 terse entry (sense 1 + labels)
verbum -v WORD              full entry (all senses, examples, etymology, translations)
verbum -e WORD              etymology only
verbum -q WORD              quirks only (archaic, countable, British…)
verbum -t de -t fr WORD     translate → German and French
verbum -l sv WORD           restrict to a source language
verbum -k WORD              fuzzy: print close spellings
verbum -r "beat, thrash"    reverse: search definitions for a word
verbum --json WORD          one JSON object per line
```

Section flags stack: `verbum -e -q WORD` shows etymology **and** quirks, nothing
else. Pipe a word list for batch lookups: `verbum < words.txt`.

**Unix behaviour.** stdout is the answer; suggestions and errors go to stderr.
Exit codes: `0` hit, `1` no match, `2` error. Colour and the pager engage only
on a terminal; `NO_COLOR` and `$PAGER` are honoured.

## How it works

- Data: kaikki.org's `wiktextract` JSONL slices, stripped to the STD field set
  (word, POS, glosses, tags, ≤2 examples, IPA, etymology, inflected forms).
- Storage: a single SQLite database with an FTS5 index
  (`unicode61 remove_diacritics 2`, so unaccented queries match accented
  headwords). Exact lookup uses a plain index; FTS drives `-r`.
- Fuzzy matching is a SQL pre-filter (initial letter + length window) ranked by
  edit distance — no C extensions, so the binary stays pure-Go and static.

## Building from source

```sh
go build ./cmd/verbum
go test ./...
```

## Data license & attribution

verbum does not bundle dictionary data; `verbum update` downloads it at runtime.
That data originates from [Wiktionary](https://www.wiktionary.org), parsed by
[wiktextract](https://github.com/tatuylonen/wiktextract) and distributed via
[kaikki.org](https://kaikki.org). Wiktionary content is licensed
**[CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/)**; any reuse
of the definitions must carry that attribution and share-alike terms.

## License

verbum's own code is licensed **MIT** — see [LICENSE](LICENSE).
