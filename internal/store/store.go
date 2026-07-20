package store

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"
	"unicode"

	"github.com/klauspost/compress/zstd"
	_ "modernc.org/sqlite"
)

// Schema is the full on-disk layout. FTS5 uses external content over
// entries(word, gloss) so the index carries no duplicate text; it is populated
// once, after bulk insert, via the 'rebuild' command.
const Schema = `
CREATE TABLE IF NOT EXISTS entries (
  id       INTEGER PRIMARY KEY,
  lang     TEXT NOT NULL,
  word     TEXT NOT NULL,
  wordfold TEXT NOT NULL,
  pos      TEXT,
  gloss    TEXT,
  data     BLOB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_lookup ON entries(wordfold, lang);
CREATE INDEX IF NOT EXISTS idx_lemma  ON entries(lang, wordfold);

CREATE VIRTUAL TABLE IF NOT EXISTS fts USING fts5(
  word, gloss,
  content='entries', content_rowid='id',
  tokenize='unicode61 remove_diacritics 2'
);

CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT);
`

// Package-level zstd codecs. EncodeAll/DecodeAll are safe for concurrent use.
var (
	zEnc, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	zDec, _ = zstd.NewReader(nil)
)

func encode(e *Entry) ([]byte, error) {
	raw, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return zEnc.EncodeAll(raw, nil), nil
}

// Decode inflates a stored data BLOB back into an Entry.
func Decode(blob []byte) (*Entry, error) {
	raw, err := zDec.DecodeAll(blob, nil)
	if err != nil {
		return nil, err
	}
	var e Entry
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// DB wraps the SQLite connection.
type DB struct {
	sql *sql.DB
}

// Open opens (or creates) the database at path.
func Open(path string) (*DB, error) {
	sdb, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	// Pin to one connection. SQLite pragmas like journal_mode are per-connection;
	// with database/sql's pool, setting them on one connection while writes land
	// on another mixes journal modes on the same file and corrupts it
	// ("database disk image is malformed"). A CLI needs no write concurrency.
	sdb.SetMaxOpenConns(1)
	return &DB{sql: sdb}, nil
}

// Close closes the underlying connection.
func (d *DB) Close() error { return d.sql.Close() }

// Init creates the schema if absent.
func (d *DB) Init() error {
	_, err := d.sql.Exec(Schema)
	return err
}

// FastPragmas trades durability for bulk-insert speed. Only used by ingest.
func (d *DB) FastPragmas() error {
	_, err := d.sql.Exec(`
		PRAGMA journal_mode = MEMORY;
		PRAGMA synchronous  = OFF;
		PRAGMA temp_store    = MEMORY;
	`)
	return err
}

// ---- Lookup -------------------------------------------------------------

func langFilter(langs []string) (string, []any) {
	langs = nonEmpty(langs)
	if len(langs) == 0 {
		return "", nil
	}
	ph := make([]string, len(langs))
	args := make([]any, len(langs))
	for i, l := range langs {
		ph[i] = "?"
		args[i] = l
	}
	return " AND lang IN (" + strings.Join(ph, ",") + ")", args
}

// Lookup returns entries whose folded headword equals the folded query,
// optionally restricted to langs. Uses idx_lookup — exact, sub-millisecond.
func (d *DB) Lookup(word string, langs []string) ([]*Entry, error) {
	where, args := langFilter(langs)
	q := "SELECT data FROM entries WHERE wordfold = ?" + where + " ORDER BY lang"
	all := append([]any{Fold(word)}, args...)
	return d.queryEntries(q, all...)
}

// Reverse runs a definition/gloss search (the -r path) via FTS5.
func (d *DB) Reverse(query string, langs []string, limit int) ([]*Entry, error) {
	match := buildMatch(query)
	if match == "" {
		return nil, nil
	}
	where, wargs := langFilter(langs)
	q := `SELECT e.data FROM fts JOIN entries e ON e.id = fts.rowid
	      WHERE fts MATCH ?` + where + ` ORDER BY bm25(fts) LIMIT ?`
	args := append([]any{match}, wargs...)
	args = append(args, limit)
	return d.queryEntries(q, args...)
}

// Candidate is a headword returned by the fuzzy pre-filter.
type Candidate struct{ Word, Fold string }

// FuzzyCandidates returns a cheap SQL pre-filtered set for edit-distance
// ranking: same initial character, length within ±2 of the query. Keeps the
// per-invocation working set tiny instead of loading every headword.
func (d *DB) FuzzyCandidates(fold string, langs []string, limit int) ([]Candidate, error) {
	runes := []rune(fold)
	if len(runes) == 0 {
		return nil, nil
	}
	first := string(runes[:1])
	n := len(runes)
	where, wargs := langFilter(langs)
	q := `SELECT DISTINCT word, wordfold FROM entries
	      WHERE substr(wordfold,1,1) = ? AND length(wordfold) BETWEEN ? AND ?` +
		where + ` LIMIT ?`
	args := append([]any{first, n - 2, n + 2}, wargs...)
	args = append(args, limit)
	rows, err := d.sql.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Candidate
	for rows.Next() {
		var c Candidate
		if err := rows.Scan(&c.Word, &c.Fold); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *DB) queryEntries(q string, args ...any) ([]*Entry, error) {
	rows, err := d.sql.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Entry
	for rows.Next() {
		var blob []byte
		if err := rows.Scan(&blob); err != nil {
			return nil, err
		}
		e, err := Decode(blob)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ---- Ingest -------------------------------------------------------------

// Loader batches inserts for one language into ~10k-row transactions.
type Loader struct {
	db    *DB
	tx    *sql.Tx
	stmt  *sql.Stmt
	n     int
	batch int
}

// BeginLoad clears any existing rows for langCode and starts a loader.
func (d *DB) BeginLoad(langCode string) (*Loader, error) {
	if _, err := d.sql.Exec("DELETE FROM entries WHERE lang = ?", langCode); err != nil {
		return nil, err
	}
	l := &Loader{db: d, batch: 10000}
	if err := l.begin(); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *Loader) begin() error {
	tx, err := l.db.sql.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(
		"INSERT INTO entries(lang, word, wordfold, pos, gloss, data) VALUES(?,?,?,?,?,?)")
	if err != nil {
		tx.Rollback()
		return err
	}
	l.tx, l.stmt = tx, stmt
	return nil
}

// Add inserts one entry, committing and reopening a transaction every batch.
func (l *Loader) Add(e *Entry) error {
	blob, err := encode(e)
	if err != nil {
		return err
	}
	if _, err := l.stmt.Exec(e.Lang, e.Word, Fold(e.Word), e.POS, e.GlossText(), blob); err != nil {
		return err
	}
	l.n++
	if l.n%l.batch == 0 {
		if err := l.tx.Commit(); err != nil {
			return err
		}
		return l.begin()
	}
	return nil
}

// Finish commits the final partial transaction. Count returns rows written.
func (l *Loader) Finish() (int, error) {
	if l.tx == nil {
		return l.n, nil
	}
	err := l.tx.Commit()
	l.tx, l.stmt = nil, nil
	return l.n, err
}

// RebuildFTS repopulates the external-content FTS index from entries. Run once
// after all languages are loaded.
func (d *DB) RebuildFTS() error {
	_, err := d.sql.Exec("INSERT INTO fts(fts) VALUES('rebuild')")
	return err
}

// SetMeta stores a key/value pair (e.g. the update timestamp).
func (d *DB) SetMeta(key, value string) error {
	_, err := d.sql.Exec(
		"INSERT INTO meta(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value",
		key, value)
	return err
}

// GetMeta reads a meta value; ok is false if absent.
func (d *DB) GetMeta(key string) (string, bool) {
	var v string
	err := d.sql.QueryRow("SELECT value FROM meta WHERE key = ?", key).Scan(&v)
	if err != nil {
		return "", false
	}
	return v, true
}

// LangCounts returns row counts per language for `update --check`.
func (d *DB) LangCounts() (map[string]int, error) {
	rows, err := d.sql.Query("SELECT lang, COUNT(*) FROM entries GROUP BY lang")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var lang string
		var n int
		if err := rows.Scan(&lang, &n); err != nil {
			return nil, err
		}
		out[lang] = n
	}
	return out, rows.Err()
}

// Updated returns the stored update time, if any.
func (d *DB) Updated() (time.Time, bool) {
	v, ok := d.GetMeta("updated")
	if !ok {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// ---- helpers ------------------------------------------------------------

func nonEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// buildMatch turns a free-text query ("beat, thrash") into a safe FTS5 MATCH
// expression: each alphanumeric term quoted, joined with AND for precision.
func buildMatch(q string) string {
	fields := strings.FieldsFunc(q, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	var terms []string
	for _, f := range fields {
		terms = append(terms, `"`+f+`"`)
	}
	return strings.Join(terms, " AND ")
}
