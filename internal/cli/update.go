package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/linusnyman/verbum/internal/ingest"
	"github.com/linusnyman/verbum/internal/paths"
	"github.com/linusnyman/verbum/internal/store"
)

// runUpdate handles `verbum update [--check] [--lang CODE ...]`.
func runUpdate(argv []string) int {
	var check bool
	var only []string
	for i := 0; i < len(argv); i++ {
		switch a := argv[i]; {
		case a == "--check":
			check = true
		case a == "--lang" || a == "-l":
			if i+1 >= len(argv) {
				fmt.Fprintln(os.Stderr, "verbum: --lang needs a code")
				return 2
			}
			i++
			only = append(only, argv[i])
		case strings.HasPrefix(a, "--lang="):
			only = append(only, strings.TrimPrefix(a, "--lang="))
		default:
			fmt.Fprintf(os.Stderr, "verbum: unknown update flag %q\n", a)
			return 2
		}
	}

	if check {
		return runCheck()
	}

	dbPath, err := paths.DBPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "verbum: "+err.Error())
		return 2
	}
	err = ingest.Run(dbPath, ingest.Options{
		Only: only,
		Log:  func(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\n", a...) },
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "verbum: update failed: "+err.Error())
		return 2
	}
	return 0
}

// runCheck reports installed data age and per-language counts (no download).
func runCheck() int {
	if !paths.DBExists() {
		fmt.Fprintln(os.Stderr, "verbum: no dictionary yet — run `verbum update`")
		return 1
	}
	dbPath, _ := paths.DBPath()
	db, err := store.Open(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "verbum: "+err.Error())
		return 2
	}
	defer db.Close()

	if t, ok := db.Updated(); ok {
		fmt.Printf("updated: %s\n", t.Format("2006-01-02 15:04"))
	} else {
		fmt.Println("updated: unknown")
	}
	counts, err := db.LangCounts()
	if err != nil {
		fmt.Fprintln(os.Stderr, "verbum: "+err.Error())
		return 2
	}
	codes := make([]string, 0, len(counts))
	for c := range counts {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	total := 0
	for _, c := range codes {
		fmt.Printf("  %-4s %d\n", c, counts[c])
		total += counts[c]
	}
	fmt.Printf("total: %d entries\n", total)
	fmt.Println("(Wiktionary dumps monthly; re-run `verbum update` to refresh)")
	return 0
}
