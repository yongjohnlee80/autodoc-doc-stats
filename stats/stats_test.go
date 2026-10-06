package stats

import (
	"reflect"
	"strings"
	"testing"

	"github.com/yongjohnlee80/autodoc/plugin"
)

const note = `---
title: Gardens of the north
tags: [gardens, tags]
---
# Gardens

The gardens of the north grow slowly. A garden there [needs](https://example.com/needs) care.

## Soil

Don’t dig the soil wet; soil packs down. See https://soil.example.org/guide for more.

` + "```" + `
# not a heading
soil soil soil
` + "```" + `
    # indented: code, not a heading
####### seven: not a heading
#hashtag, not a heading`

// TestOfMeasuresTheProseNotTheFrontMatter: the counts, with the front matter left out, headings
// outside code fences only, and link targets not counted as words.
func TestOfMeasuresTheProseNotTheFrontMatter(t *testing.T) {
	s := Of(note)
	if s.Sections != 2 {
		t.Errorf("sections = %d, want 2 (# Gardens, ## Soil)", s.Sections)
	}
	if s.Lines != 15 {
		t.Errorf("lines = %d, want 15: the front matter's 4 are not the note's", s.Lines)
	}
	top := map[string]int{}
	for _, w := range s.Top {
		top[w.Word] = w.N
	}
	// front matter (title), a link's target (https, example, com, org, guide) and stop words (the, of,
	// a, don't, for, more) are never listed
	for _, w := range []string{"title", "https", "example", "com", "org", "guide", "the", "of", "a", "don't", "for", "more"} {
		if n, ok := top[w]; ok {
			t.Errorf("%q is listed (%d)", w, n)
		}
	}
	// soil: three in the prose, the heading's among them; the three in the fence are words, but
	// not frequent ones. "needs" is a link's text, which is prose.
	if top["soil"] != 3 || top["gardens"] != 2 || top["garden"] != 1 || top["needs"] != 1 {
		t.Errorf("top = %v, want soil 3, gardens 2, garden 1, needs 1", top)
	}
}

// TestFrequentWordsSortByCountThenWord: ties sort by the word, so the list does not jump as the
// note is edited.
func TestFrequentWordsSortByCountThenWord(t *testing.T) {
	got := Of("pear apple pear fig apple kiwi").Top
	want := []Count{{"apple", 2}, {"pear", 2}, {"fig", 1}, {"kiwi", 1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("top = %v, want %v", got, want)
	}
}

// TestWordsKeepTheirJoinsAndApostrophes: don't and x-ray are one word each; a typographic
// apostrophe is the same word as a typed one, so it is a stop word too.
func TestWordsKeepTheirJoinsAndApostrophes(t *testing.T) {
	if got := words("Don’t x-ray it - twice' 'quoted' 3.14"); !reflect.DeepEqual(got,
		[]string{"don't", "x-ray", "it", "twice", "quoted", "3", "14"}) {
		t.Errorf("words = %q", got)
	}
	if s := Of("Don’t don't DON'T"); len(s.Top) != 0 || s.Words != 3 {
		t.Errorf("words %d, top %v: each is don't, a stop word", s.Words, s.Top)
	}
}

// TestNumbersAndOneLetterAreNotFrequent: a word needs two characters and a letter to be listed.
func TestNumbersAndOneLetterAreNotFrequent(t *testing.T) {
	s := Of("x 2026 2026 v2 v2 q")
	if s.Words != 6 || len(s.Top) != 1 || s.Top[0] != (Count{"v2", 2}) {
		t.Errorf("words %d, top %v: want 6, [v2 2]", s.Words, s.Top)
	}
}

// TestCharactersCountWhatShowsAsOne: é written as e and a combining accent, a family emoji and a
// flag are one character each, as the editor's cursor counts them.
func TestCharactersCountWhatShowsAsOne(t *testing.T) {
	s := Of("é👨‍👩‍👧🇰🇷\n한국")
	if s.Characters != 5 || s.Lines != 2 {
		t.Errorf("characters %d, lines %d: want 5, 2", s.Characters, s.Lines)
	}
}

func TestReadingMinutesRoundUp(t *testing.T) {
	for words, want := range map[int]int{0: 0, 1: 1, 230: 1, 231: 2, 461: 3} {
		if got := (Stats{Words: words}).ReadingMinutes(); got != want {
			t.Errorf("%d words: %d min, want %d", words, got, want)
		}
	}
}

// TestFrontMatterNeedsItsClosingLine: a note that opens with --- and never closes it has no front
// matter, so nothing of it is dropped.
func TestFrontMatterNeedsItsClosingLine(t *testing.T) {
	if s := Of("---\ntitle: open\nprose here"); s.Words != 4 || s.Lines != 3 {
		t.Errorf("words %d, lines %d: want 4, 3", s.Words, s.Lines)
	}
	if s := Of("---\ntitle: x\n---"); s.Words != 0 || s.Lines != 0 {
		t.Errorf("only front matter: words %d, lines %d", s.Words, s.Lines)
	}
	if s := Of("---\ntitle: x\n...\nbody"); s.Words != 1 {
		t.Errorf("closed by ...: words %d, want 1", s.Words)
	}
}

func pos(l, c int) plugin.Position { return plugin.Position{Line: l, Col: c} }

// TestSelectedCutsByWhatShowsAsOne: columns count grapheme clusters, from 1, the end just after the
// last; a selection made backwards is the same text; a line-wise one runs to its line's end.
func TestSelectedCutsByWhatShowsAsOne(t *testing.T) {
	text := "café au lait\n👨‍👩‍👧 family\nlast line"
	cases := []struct {
		name string
		r    plugin.Range
		want string
	}{
		{"one cluster with its accent", plugin.Range{Start: pos(1, 4), End: pos(1, 5)}, "é"},
		{"a word after it", plugin.Range{Start: pos(1, 6), End: pos(1, 8)}, "au"},
		{"the family is one column", plugin.Range{Start: pos(2, 1), End: pos(2, 2)}, "👨‍👩‍👧"},
		{"across lines", plugin.Range{Start: pos(1, 9), End: pos(2, 2)}, "lait\n👨‍👩‍👧"},
		{"backwards", plugin.Range{Start: pos(2, 2), End: pos(1, 9)}, "lait\n👨‍👩‍👧"},
		{"line-wise", plugin.Range{Start: pos(2, 1), End: pos(3, 10)}, "👨‍👩‍👧 family\nlast line"},
		{"past the end", plugin.Range{Start: pos(3, 6), End: pos(9, 1)}, "line"},
		{"empty", plugin.Range{Start: pos(1, 3), End: pos(1, 3)}, ""},
	}
	for _, c := range cases {
		if got := Selected(text, []plugin.Range{c.r}); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	two := Selected(text, []plugin.Range{{Start: pos(1, 1), End: pos(1, 5)}, {Start: pos(3, 1), End: pos(3, 5)}})
	if two != "café\nlast" {
		t.Errorf("two ranges: %q", two)
	}
}

// TestStopWordsAreLowerCase guards the list's own shape: a capital would never match a lower-cased
// word.
func TestStopWordsAreLowerCase(t *testing.T) {
	if len(stop) < 150 {
		t.Errorf("%d stop words: the list did not load", len(stop))
	}
	for w := range stop {
		if w != strings.ToLower(w) {
			t.Errorf("%q is not lower-case", w)
		}
	}
}
