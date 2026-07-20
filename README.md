# vox

An offline, `man`-style terminal dictionary for reading across languages —
English, Swedish, German, French, Spanish, Latin, Greek, and Ancient Greek.
Built for looking up **etymology**, **word quirks** (labels like *archaic*,
*countable*), and **translations**, with fuzzy spell matching when you only
half-remember a word. Unix-composable: quiet, pipe-friendly, exit codes.

```
$ vox pattern
pattern  noun  /ˈpæt.ən/, /ˈpæt.ɚn/
  (countable)
  1. A design, motif or decorative arrangement.
```

## Install

**Homebrew**

```sh
brew install linusnyman/vox/vox
```

**Go**

```sh
go install github.com/linusnyman/vox/cmd/vox@latest
```

Or grab a binary from the [releases](https://github.com/linusnyman/vox/releases).

## Build the dictionary

The binary ships **without data**. Build the local database once (downloads
~1.5 GB of parsed Wiktionary data from [kaikki.org](https://kaikki.org),
streamed and stripped on the fly; final DB ~1.6 GB):

```sh
vox update                 # all 8 languages
vox update --lang sv       # or just one, to try it out
vox update --check         # show installed data age + counts
```

Data lives under `${XDG_DATA_HOME:-~/.local/share}/vox/vox.db`.

## Use

```
vox WORD                 terse entry (sense 1 + labels)
vox -v WORD              full entry (all senses, examples, etymology, translations)
vox -e WORD              etymology only
vox -q WORD              quirks only (archaic, countable, British…)
vox -t de -t fr WORD     translate → German and French
vox -l sv WORD           restrict to a source language
vox -k WORD              fuzzy: print close spellings
vox -r "beat, thrash"    reverse: search definitions for a word
vox --json WORD          one JSON object per line
```

Section flags stack: `vox -e -q WORD` shows etymology **and** quirks, nothing
else. Pipe a word list for batch lookups: `vox < words.txt`.

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
go build ./cmd/vox
go test ./...
```

## Data license & attribution

vox does not bundle dictionary data; `vox update` downloads it at runtime.
That data originates from [Wiktionary](https://www.wiktionary.org), parsed by
[wiktextract](https://github.com/tatuylonen/wiktextract) and distributed via
[kaikki.org](https://kaikki.org). Wiktionary content is licensed
**[CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/)**; any reuse
of the definitions must carry that attribution and share-alike terms.

## License

vox's own code is licensed **MIT** — see [LICENSE](LICENSE).
