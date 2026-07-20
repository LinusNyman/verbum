// Package fuzzy ranks spelling candidates by edit distance.
//
// The plan named a BK-tree, but vox is a per-invocation CLI with no resident
// process: building a tree over every headword on each run would cost seconds
// and hundreds of MB for a feature that only fires on a miss. Instead the store
// pre-filters candidates in SQL (same initial character, length ±2) and this
// package ranks that small set with plain Levenshtein — same result, negligible
// cost. Swap in a persistent BK-tree only if vox ever grows a long-lived daemon.
package fuzzy

// Levenshtein returns the edit distance between a and b (rune-wise), using a
// single-row dynamic-programming buffer.
func Levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 {
		return len(rb)
	}
	if len(rb) == 0 {
		return len(ra)
	}
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min3(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

// Match is a scored candidate.
type Match struct {
	Word     string
	Distance int
}

// Candidate pairs a display word with its folded form (what we score against).
type Candidate struct{ Word, Fold string }

// Rank scores candidates against the folded query and returns the closest n,
// dropping anything past a length-scaled distance threshold and de-duplicating
// display words. Results are ordered by distance, then by word length.
func Rank(queryFold string, cands []Candidate, n int) []Match {
	qr := []rune(queryFold)
	maxDist := 2
	if len(qr) >= 8 {
		maxDist = 3
	}
	seen := map[string]bool{}
	var matches []Match
	for _, c := range cands {
		if c.Fold == queryFold {
			continue // exact — not a suggestion
		}
		d := Levenshtein(queryFold, c.Fold)
		if d > maxDist || seen[c.Word] {
			continue
		}
		seen[c.Word] = true
		matches = append(matches, Match{Word: c.Word, Distance: d})
	}
	sortMatches(matches)
	if len(matches) > n {
		matches = matches[:n]
	}
	return matches
}

// sortMatches: insertion sort — candidate lists are already small after the
// SQL pre-filter, so this avoids pulling in sort for a hot, tiny slice.
func sortMatches(m []Match) {
	for i := 1; i < len(m); i++ {
		for j := i; j > 0 && less(m[j], m[j-1]); j-- {
			m[j], m[j-1] = m[j-1], m[j]
		}
	}
}

func less(a, b Match) bool {
	if a.Distance != b.Distance {
		return a.Distance < b.Distance
	}
	return len(a.Word) < len(b.Word)
}
