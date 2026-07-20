// Package ingest builds the vox database from kaikki.org Wiktionary slices:
// stream each language, strip during download (never landing the raw 7.5 GB),
// load into SQLite in ~10k-row transactions, then rebuild FTS once at the end.
package ingest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/linusnyman/vox/internal/paths"
	"github.com/linusnyman/vox/internal/store"
)

// estBytesPerLang is a rough STD-tier on-disk estimate (report §2, ±20%),
// used only for the pre-flight free-space guard.
var estBytesPerLang = map[string]uint64{
	"en": 420 << 20, "la": 220 << 20, "de": 140 << 20, "es": 200 << 20,
	"fr": 90 << 20, "grc": 50 << 20, "sv": 55 << 20, "el": 30 << 20,
}

// Options configures an update run.
type Options struct {
	Only    []string // language codes to build; empty = all
	Fetcher func(url string) (io.ReadCloser, error)
	Log     func(format string, a ...any) // progress sink (stderr)
}

func (o Options) logf(format string, a ...any) {
	if o.Log != nil {
		o.Log(format, a...)
	}
}

// selected resolves Options.Only into Lang structs (all when empty).
func (o Options) selected() ([]Lang, error) {
	if len(o.Only) == 0 {
		return Langs, nil
	}
	var out []Lang
	for _, code := range o.Only {
		l, ok := ByCode(code)
		if !ok {
			return nil, fmt.Errorf("unknown language code %q", code)
		}
		out = append(out, l)
	}
	return out, nil
}

// httpFetch streams a URL body.
func httpFetch(url string) (io.ReadCloser, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return resp.Body, nil
}

// Run executes the update. dbPath is the SQLite file to build.
func Run(dbPath string, opts Options) error {
	langs, err := opts.selected()
	if err != nil {
		return err
	}
	if opts.Fetcher == nil {
		opts.Fetcher = httpFetch
	}

	if err := preflightDisk(dbPath, langs, opts); err != nil {
		return err
	}

	db, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Init(); err != nil {
		return err
	}
	if err := db.FastPragmas(); err != nil {
		return err
	}

	total := 0
	start := time.Now()
	for _, l := range langs {
		n, err := loadLang(db, l, opts)
		if err != nil {
			return fmt.Errorf("%s: %w", l.Name, err)
		}
		total += n
		opts.logf("  %-14s %d entries", l.Name, n)
	}

	opts.logf("building search index…")
	if err := db.RebuildFTS(); err != nil {
		return fmt.Errorf("build FTS: %w", err)
	}
	if err := db.SetMeta("updated", time.Now().Format(time.RFC3339)); err != nil {
		return err
	}
	opts.logf("done: %d entries in %s", total, time.Since(start).Round(time.Second))
	return nil
}

// loadLang streams one slice through strip → Loader.
func loadLang(db *store.DB, l Lang, opts Options) (int, error) {
	opts.logf("fetching %s …", l.Name)
	body, err := opts.Fetcher(l.URL())
	if err != nil {
		return 0, err
	}
	defer body.Close()

	loader, err := db.BeginLoad(l.Code)
	if err != nil {
		return 0, err
	}

	r := bufio.NewReaderSize(body, 1<<20)
	for {
		buf, err := r.ReadBytes('\n')
		if len(buf) > 0 {
			var raw rawEntry
			if jerr := json.Unmarshal(buf, &raw); jerr != nil {
				continue // skip malformed line, keep going
			}
			if e := strip(&raw); e != nil {
				if aerr := loader.Add(e); aerr != nil {
					return 0, aerr
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
	}
	return loader.Finish()
}

// preflightDisk aborts before any download if the target filesystem clearly
// lacks room. Skipped silently on platforms without a free-space syscall.
func preflightDisk(dbPath string, langs []Lang, opts Options) error {
	free, ok := paths.FreeBytes(dbPath)
	if !ok {
		return nil
	}
	var need uint64
	for _, l := range langs {
		need += estBytesPerLang[l.Code]
	}
	need += need / 3 // headroom for indexes + WAL
	if free < need {
		return fmt.Errorf(
			"not enough disk: need ~%d MB, have %d MB free (build fewer languages with --lang)",
			need>>20, free>>20)
	}
	opts.logf("disk ok: ~%d MB needed, %d MB free", need>>20, free>>20)
	return nil
}
