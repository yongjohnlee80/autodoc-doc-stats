// Package stats measures a Markdown note: its words, characters, lines and sections, how long it
// takes to read, and the words it uses most, without the stop words.
package stats

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/yongjohnlee80/autodoc/plugin"
	"github.com/yongjohnlee80/golib/tui"
)

// WordsPerMinute is the reading speed the reading time is measured at: an adult's, silently, in
// English.
const WordsPerMinute = 230

// Stats are a text's measures.
type Stats struct {
	Words      int
	Characters int // what shows as one character (grapheme clusters), line breaks not counted
	Lines      int
	Sections   int // Markdown headings outside code fences
	Top        []Count
}

// Count is a word and how often it is used.
type Count struct {
	Word string
	N    int
}

// ReadingMinutes is the time the text takes to read, rounded up: 0 only for no words at all.
func (s Stats) ReadingMinutes() int { return (s.Words + WordsPerMinute - 1) / WordsPerMinute }

// Of measures text. A note's front matter (a YAML block that opens with --- on its first line) is
// not counted: it is the note's metadata, not its prose.
func Of(text string) Stats {
	text = body(text)
	var s Stats
	if text == "" {
		return s
	}
	lines := strings.Split(text, "\n")
	s.Lines = len(lines)
	fence := ""
	counts := map[string]int{}
	for _, l := range lines {
		s.Characters += chars(l)
		trimmed := strings.TrimLeft(l, " ")
		if f := fenceOf(trimmed); f != "" && (fence == "" || strings.HasPrefix(trimmed, fence)) {
			if fence == "" {
				fence = f
			} else {
				fence = ""
			}
			continue
		}
		if fence == "" && len(l)-len(trimmed) < 4 && heading(trimmed) {
			s.Sections++
		}
		for _, w := range words(prose(l)) {
			s.Words++
			if fence == "" && frequent(w) {
				counts[w]++
			}
		}
	}
	for w, n := range counts {
		s.Top = append(s.Top, Count{w, n})
	}
	slices.SortFunc(s.Top, func(a, b Count) int {
		return cmp.Or(cmp.Compare(b.N, a.N), cmp.Compare(a.Word, b.Word))
	})
	return s
}

// body is text without its front matter.
func body(text string) string {
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return text
	}
	for i := 0; i < len(rest); {
		end := strings.IndexByte(rest[i:], '\n')
		line := rest[i:]
		if end >= 0 {
			line = rest[i : i+end]
		}
		if line == "---" || line == "..." {
			if end < 0 {
				return ""
			}
			return rest[i+end+1:]
		}
		if end < 0 {
			break
		}
		i += end + 1
	}
	return text // no closing line: not front matter
}

// fenceOf is the fence a line opens or closes (``` or ~~~, three or more), or "".
func fenceOf(l string) string {
	for _, c := range []string{"`", "~"} {
		n := len(l) - len(strings.TrimLeft(l, c))
		if n >= 3 {
			return strings.Repeat(c, 3)
		}
	}
	return ""
}

// heading says l is an ATX heading: one to six #, then a space or the line's end.
func heading(l string) bool {
	n := len(l) - len(strings.TrimLeft(l, "#"))
	return n >= 1 && n <= 6 && (len(l) == n || l[n] == ' ' || l[n] == '\t')
}

func chars(l string) int {
	n := 0
	for range tui.Graphemes(l) {
		n++
	}
	return n
}

// links are a Markdown link's target, ](…), and a bare URL: where a link goes is not the note's
// prose.
var links = regexp.MustCompile(`\]\([^)]*\)|<?\b[a-z][a-z0-9+.-]*://[^\s>)]*>?`)

// prose is l without its links' targets.
func prose(l string) string { return links.ReplaceAllString(l, "] ") }

// words are l's words, lower-cased: runs of letters and digits, with an apostrophe or hyphen
// inside a word kept ("don't", "x-ray").
func words(l string) []string {
	var out []string
	rs := []rune(l)
	for i := 0; i < len(rs); {
		if !wordRune(rs[i]) {
			i++
			continue
		}
		j := i + 1
		for j < len(rs) && (wordRune(rs[j]) || joiner(rs[j]) && j+1 < len(rs) && wordRune(rs[j+1])) {
			j++
		}
		out = append(out, strings.ReplaceAll(strings.ToLower(string(rs[i:j])), "’", "'"))
		i = j
	}
	return out
}

func wordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.Is(unicode.Mn, r)
}
func joiner(r rune) bool { return r == '\'' || r == '’' || r == '-' }

// frequent says w may be among the most frequent words: two characters or more, a letter among
// them, and not a stop word.
func frequent(w string) bool {
	if chars(w) < 2 || stop[w] {
		return false
	}
	return strings.IndexFunc(w, unicode.IsLetter) >= 0
}

// Selected is the text of ranges in text, in order, joined by line breaks. Positions count from 1;
// a column counts what shows as one character, as the editor's cursor does; End is just after the
// range's last character.
func Selected(text string, ranges []plugin.Range) string {
	lines := strings.Split(text, "\n")
	var parts []string
	for _, r := range ranges {
		parts = append(parts, slice(lines, r))
	}
	return strings.Join(parts, "\n")
}

func slice(lines []string, r plugin.Range) string {
	s, e := r.Start, r.End
	if e.Line < s.Line || e.Line == s.Line && e.Col < s.Col {
		s, e = e, s
	}
	var b strings.Builder
	first := max(s.Line, 1)
	for ln := first; ln <= min(e.Line, len(lines)); ln++ {
		if ln > first {
			b.WriteByte('\n')
		}
		from, to := 1, -1 // to < 0: the line's end
		if ln == s.Line {
			from = s.Col
		}
		if ln == e.Line {
			to = e.Col
		}
		col := 1
		for g := range tui.Graphemes(lines[ln-1]) {
			if col >= from && (to < 0 || col < to) {
				b.WriteString(g)
			}
			col++
		}
	}
	return b.String()
}
