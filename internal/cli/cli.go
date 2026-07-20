// Package cli parses arguments and drives a single vox invocation. It owns the
// Unix contract: stdout is the answer, suggestions/errors go to stderr, and the
// process exit code is 0 (hit) / 1 (no match) / 2 (usage or runtime error).
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"

	"github.com/linusnyman/vox/internal/fuzzy"
	"github.com/linusnyman/vox/internal/paths"
	"github.com/linusnyman/vox/internal/render"
	"github.com/linusnyman/vox/internal/store"
)

const version = "0.1.0"

// Main is the process entry point; it returns the exit code.
func Main(argv []string) int {
	if len(argv) > 0 && argv[0] == "update" {
		return runUpdate(argv[1:])
	}

	o, err := parse(argv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "vox: "+err.Error())
		return 2
	}
	if o.Help {
		fmt.Print(usage)
		return 0
	}
	if o.Version {
		fmt.Println("vox " + version)
		return 0
	}

	if !paths.DBExists() {
		fmt.Fprintln(os.Stderr, "vox: no dictionary yet — run `vox update` to build it")
		return 2
	}
	dbPath, err := paths.DBPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "vox: "+err.Error())
		return 2
	}
	db, err := store.Open(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "vox: "+err.Error())
		return 2
	}
	defer db.Close()

	outTTY := isatty.IsTerminal(os.Stdout.Fd())
	ropts := render.Options{
		Verbose:    o.Verbose,
		Etym:       o.Etym,
		Quirks:     o.Quirks,
		Color:      outTTY && os.Getenv("NO_COLOR") == "" && !o.JSON,
		TransLangs: o.Trans,
	}

	// Batch mode: no query given and stdin is piped → one lookup per line.
	if len(o.Args) == 0 && !isatty.IsTerminal(os.Stdin.Fd()) {
		return runBatch(db, o, ropts, outTTY)
	}
	if len(o.Args) == 0 {
		fmt.Fprintln(os.Stderr, "vox: no word given (try `vox -h`)")
		return 2
	}

	return lookup(db, o.query(), o, ropts, outTTY)
}

func runBatch(db *store.DB, o Options, ropts render.Options, outTTY bool) int {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	worst := 0
	for sc.Scan() {
		word := strings.TrimSpace(sc.Text())
		if word == "" {
			continue
		}
		if code := lookup(db, word, o, ropts, false); code == 2 {
			worst = 2
		}
	}
	_ = outTTY
	return worst
}

// lookup handles one query end to end and returns its exit code.
func lookup(db *store.DB, q string, o Options, ropts render.Options, outTTY bool) int {
	if o.Reverse {
		return reverse(db, q, o, outTTY)
	}

	entries, err := db.Lookup(q, o.Langs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "vox: "+err.Error())
		return 2
	}

	if len(entries) == 0 {
		sugg := suggest(db, q, o.Langs)
		if o.Fuzzy { // -k : the candidate list IS the output (stdout)
			if len(sugg) == 0 {
				fmt.Fprintf(os.Stderr, "vox: no matches near %q\n", q)
				return 1
			}
			fmt.Println(strings.Join(sugg, "\n"))
			return 0
		}
		if len(sugg) > 0 {
			fmt.Fprintf(os.Stderr, "vox: %q not found. did you mean: %s\n", q, strings.Join(sugg, ", "))
		} else {
			fmt.Fprintf(os.Stderr, "vox: %q not found\n", q)
		}
		return 1
	}

	// Resolve inflected-form stubs to their lemma and show both (report §5).
	entries = withLemmas(db, entries)

	if o.JSON {
		out, err := render.JSONLines(entries)
		if err != nil {
			fmt.Fprintln(os.Stderr, "vox: "+err.Error())
			return 2
		}
		fmt.Print(out)
		return 0
	}

	if err := render.Page(render.Entries(entries, ropts), outTTY); err != nil {
		fmt.Fprintln(os.Stderr, "vox: "+err.Error())
		return 2
	}
	return 0
}

// withLemmas appends the lemma entry for any form-of stub, de-duplicated.
func withLemmas(db *store.DB, entries []*store.Entry) []*store.Entry {
	seen := map[string]bool{}
	for _, e := range entries {
		seen[e.Lang+"\x00"+e.Word] = true
	}
	var extra []*store.Entry
	for _, e := range entries {
		lemma, ok := e.IsFormOf()
		if !ok {
			continue
		}
		lems, err := db.Lookup(lemma, []string{e.Lang})
		if err != nil {
			continue
		}
		for _, l := range lems {
			key := l.Lang + "\x00" + l.Word
			if !seen[key] {
				seen[key] = true
				extra = append(extra, l)
			}
		}
	}
	return append(entries, extra...)
}

func reverse(db *store.DB, q string, o Options, outTTY bool) int {
	entries, err := db.Reverse(q, o.Langs, 20)
	if err != nil {
		fmt.Fprintln(os.Stderr, "vox: "+err.Error())
		return 2
	}
	if len(entries) == 0 {
		fmt.Fprintf(os.Stderr, "vox: nothing matches %q\n", q)
		return 1
	}
	if o.JSON {
		out, _ := render.JSONLines(entries)
		fmt.Print(out)
		return 0
	}
	ropts := render.Options{Color: outTTY && os.Getenv("NO_COLOR") == ""}
	if render.Page(render.Entries(entries, ropts), outTTY) != nil {
		return 2
	}
	return 0
}

// suggest returns fuzzy spelling candidates for a miss.
func suggest(db *store.DB, q string, langs []string) []string {
	fold := store.Fold(q)
	cands, err := db.FuzzyCandidates(fold, langs, 20000)
	if err != nil {
		return nil
	}
	fc := make([]fuzzy.Candidate, len(cands))
	for i, c := range cands {
		fc[i] = fuzzy.Candidate{Word: c.Word, Fold: c.Fold}
	}
	var out []string
	for _, m := range fuzzy.Rank(fold, fc, 7) {
		out = append(out, m.Word)
	}
	return out
}
