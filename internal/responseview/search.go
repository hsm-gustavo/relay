package responseview

import (
	"unicode"
	"unicode/utf8"
)

const (
	MaxMatches    = 10000
	MaxQueryRunes = 256
	PageRunes     = 4096
	PageColumns   = 120
	PageLines     = 60
)

type Match struct{ Start, End int } // Rune offsets in TextFor(mode).
type SearchResult struct {
	Matches []Match
	Limited bool
	Note    string
}

// Find uses a linear-time search with Unicode simple case folding. Limits apply
// to both input and result count; no regular expressions are interpreted.
func Find(text, query string) SearchResult {
	result := SearchResult{}
	if query == "" {
		return result
	}
	if len(text) > TextLimit {
		text = text[:TextLimit]
		for !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
		result.Limited = true
		result.Note = "Search limited to the first 256 KiB."
	}
	if len(query) > MaxQueryRunes*utf8.UTFMax {
		result.Limited = true
		result.Note = "Search terms are limited to 256 characters."
		return result
	}
	needle := []rune(query)
	if len(needle) > MaxQueryRunes {
		result.Limited = true
		result.Note = "Search terms are limited to 256 characters."
		return result
	}
	for i := range needle {
		needle[i] = foldRune(needle[i])
	}
	prefix := make([]int, len(needle))
	for i, j := 1, 0; i < len(needle); i++ {
		for j > 0 && needle[i] != needle[j] {
			j = prefix[j-1]
		}
		if needle[i] == needle[j] {
			j++
		}
		prefix[i] = j
	}
	index, j := 0, 0
	for _, r := range text {
		r = foldRune(r)
		for j > 0 && r != needle[j] {
			j = prefix[j-1]
		}
		if r == needle[j] {
			j++
		}
		if j == len(needle) {
			if len(result.Matches) == MaxMatches {
				result.Limited = true
				result.Note = "Showing the first 10,000 matches."
				break
			}
			result.Matches = append(result.Matches, Match{index - len(needle) + 1, index + 1})
			j = 0
		}
		index++
	}
	return result
}

func Navigate(current, delta, count int) int {
	if count == 0 {
		return -1
	}
	if current < 0 {
		if delta < 0 {
			return count - 1
		}
		return 0
	}
	return ((current+delta)%count + count) % count
}

type Page struct {
	Text       string
	Start, End int
	More       bool
}

// Window returns an unchanged contiguous slice of the textual representation.
// Long raw lines are paged instead of being rewritten with synthetic newlines.
func Window(text string, start int) Page {
	if start < 0 {
		start = 0
	}
	runeIndex, byteStart := 0, len(text)
	for offset := range text {
		if runeIndex == start {
			byteStart = offset
			break
		}
		runeIndex++
	}
	if start == 0 {
		byteStart = 0
	}
	end := byteStart
	count, column, lines, maxWidth := 0, 0, 1, 0
	for offset, r := range text[byteStart:] {
		width := 1
		if r > 255 {
			width = 2
		}
		if r == '\t' {
			width = 8 - column%8
		}
		nextColumn, nextLines := column+width, lines
		if r == '\n' {
			nextColumn = 0
			nextLines++
		}
		nextWidth := maxWidth
		if nextColumn > nextWidth {
			nextWidth = nextColumn
		}
		if count >= PageRunes || nextLines > PageLines || nextWidth*nextLines > PageRunes*2 {
			break
		}
		_, size := utf8.DecodeRuneInString(text[byteStart+offset:])
		end = byteStart + offset + size
		count++
		column = nextColumn
		lines = nextLines
		maxWidth = nextWidth
	}

	return Page{Text: text[byteStart:end], Start: start, End: start + count, More: end < len(text)}
}

func foldRune(r rune) rune {
	smallest := r
	for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
		if next < smallest {
			smallest = next
		}
	}
	return smallest
}
