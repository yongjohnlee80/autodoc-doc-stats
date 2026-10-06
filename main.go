// autodoc-doc-stats is an AutoDoc plugin: a card beside the page that measures the open note as it
// is edited. It shows the words, characters, lines and sections, the reading time, and the words
// the note uses most, without the stop words, for the whole note or for the selection.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/yongjohnlee80/autodoc/plugin"
	"github.com/yongjohnlee80/golib/tui"

	"github.com/yongjohnlee80/autodoc-doc-stats/stats"
)

// card is the plugin. Serve calls it on one goroutine, so it needs no lock.
type card struct {
	peer    *plugin.Peer
	w, h    int
	theme   plugin.Theme
	doc     *plugin.Document // nil until the first arrives
	st      stats.Stats
	part    bool // the selection is measured, not the whole note
	scroll  int  // the first frequent word shown
	focused bool
}

func (c *card) Open(p *plugin.Peer, o plugin.Open) {
	c.peer, c.w, c.h, c.theme = p, o.Width, o.Height, o.Theme
	c.draw()
}

func (c *card) Resize(w, h int) { c.w, c.h = w, h; c.draw() }

func (c *card) Theme(t plugin.Theme) { c.theme = t; c.draw() }

func (c *card) Close() {}

// Document is the feed: the note as it stands, after each pause in editing.
func (c *card) Document(d plugin.Document) {
	if c.doc == nil || d.Version != c.doc.Version || d.Path != c.doc.Path {
		c.scroll = 0
	}
	c.doc = &d
	text := d.Text
	c.part = false
	if s := stats.Selected(d.Text, d.Selection); len(d.Selection) > 0 && s != "" {
		text, c.part = s, true
	}
	c.st = stats.Of(text)
	c.draw()
}

// Focus is the card taking the keys, or giving them back on Esc.
func (c *card) Focus(focused bool) { c.focused = focused; c.draw() }

// Command is a manifest command. The only one, toggle, is AutoDoc's own and never sent.
func (c *card) Command(string) {}

// Key scrolls the frequent words while the card has the keys.
func (c *card) Key(k plugin.Key) {
	page := max(c.listRows()-1, 1)
	switch k.Key {
	case "Up", "k":
		c.scroll--
	case "Down", "j":
		c.scroll++
	case "PageUp":
		c.scroll -= page
	case "PageDown":
		c.scroll += page
	case "Home", "g":
		c.scroll = 0
	case "End", "G":
		c.scroll = len(c.st.Top)
	default:
		return
	}
	c.draw()
}

// the rows above the frequent words, and the row below them while focused
const listTop = 9

func (c *card) listRows() int {
	n := c.h - listTop
	if c.focused {
		n--
	}
	return max(n, 0)
}

func (c *card) draw() {
	if c.peer == nil {
		return
	}
	f := plugin.NewFrame(c.w, c.h)
	accent := plugin.Style{FG: c.color("document.cursor", "cyan"), Bold: true}
	muted := plugin.Style{FG: c.color("document.lineNumber", "gray")}
	plain := plugin.Style{}

	switch {
	case c.doc == nil:
		c.line(f, 0).put("Waiting for the note…", muted)
	case c.doc.TooLarge:
		c.line(f, 0).put(elideLeft(name(c.doc), c.w), accent)
		c.line(f, 2).put("Too large to measure: over 1 MiB.", muted)
	case c.doc.Path == "" && c.doc.Text == "":
		c.line(f, 0).put("No note open.", muted)
	default:
		c.line(f, 0).put(elideLeft(name(c.doc), c.w), accent)
		scope := "the whole note"
		if c.part {
			scope = "the selection"
		}
		c.line(f, 1).put(scope, muted)
		rows := []struct{ label, value string }{
			{"Words", thousands(c.st.Words)},
			{"Characters", thousands(c.st.Characters)},
			{"Lines", thousands(c.st.Lines)},
			{"Sections", thousands(c.st.Sections)},
			{"Reading", minutes(c.st.ReadingMinutes())},
		}
		for i, r := range rows {
			c.line(f, 3+i).pair(r.label, plain, r.value, plain)
		}
		c.drawTop(f, accent, muted)
	}
	if c.focused {
		c.line(f, c.h-1).put("↑↓ scroll · Esc back to the note", muted)
	}
	_ = c.peer.Frame(f)
}

func (c *card) drawTop(f *plugin.Frame, accent, muted plugin.Style) {
	c.line(f, listTop-1).put("Most frequent", accent)
	n := c.listRows()
	if len(c.st.Top) == 0 {
		if n > 0 {
			c.line(f, listTop).put("no words yet", muted)
		}
		return
	}
	c.scroll = max(min(c.scroll, len(c.st.Top)-n), 0)
	for i := range n {
		at := c.scroll + i
		if at >= len(c.st.Top) {
			break
		}
		w := c.st.Top[at]
		count := strconv.Itoa(w.N)
		c.line(f, listTop+i).pair(elideRight(w.Word, c.w-tui.StringWidth(count)-3), plugin.Style{}, count, muted)
	}
}

// color is the theme's colour by its dotted name, or fallback.
func (c *card) color(name, fallback string) string {
	if v := c.theme.Colors[name]; v != "" {
		return v
	}
	return fallback
}

// name is the note's path, or what stands for it.
func name(d *plugin.Document) string {
	if d.Path == "" {
		return "a draft, not saved"
	}
	return d.Path
}

func minutes(n int) string {
	if n == 1 {
		return "1 min"
	}
	return strconv.Itoa(n) + " min"
}

// thousands is n with its thousands apart: 12,345.
func thousands(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// ---------------------------------------------------------------- one row

// line writes a frame's row y by what it shows: a wide character (CJK, most emoji) is one cell of
// the frame that takes two columns on the screen, so the cells and the columns are counted apart.
type line struct {
	f          *plugin.Frame
	y, cell, w int
	col        int
}

func (c *card) line(f *plugin.Frame, y int) *line { return &line{f: f, y: y, w: c.w} }

// put writes s from the current column, as much as fits.
func (l *line) put(s string, st plugin.Style) *line {
	for g := range tui.Graphemes(s) {
		wd := tui.StringWidth(g)
		if l.col+wd > l.w {
			l.col = l.w
			break
		}
		for _, r := range g {
			l.f.Set(l.cell, l.y, r, st)
			l.cell++
		}
		l.col += wd
	}
	return l
}

// pair writes label on the left and value against the right edge, a column in from it.
func (l *line) pair(label string, lst plugin.Style, value string, vst plugin.Style) {
	l.put(" "+label, lst)
	at := l.w - 1 - tui.StringWidth(value)
	for l.col < at {
		l.put(" ", plugin.Style{})
	}
	l.put(value, vst)
}

// elideLeft fits s in width columns by dropping its start: a path's tail says which note it is.
func elideLeft(s string, width int) string {
	if tui.StringWidth(s) <= width {
		return s
	}
	var gs []string
	for g := range tui.Graphemes(s) {
		gs = append(gs, g)
	}
	for i := range gs {
		rest := "…" + join(gs[i+1:])
		if tui.StringWidth(rest) <= width {
			return rest
		}
	}
	return ""
}

// elideRight fits s in width columns by dropping its end.
func elideRight(s string, width int) string {
	if tui.StringWidth(s) <= width {
		return s
	}
	out, wd := "", 1 // the …
	for g := range tui.Graphemes(s) {
		gw := tui.StringWidth(g)
		if wd+gw > width {
			break
		}
		out += g
		wd += gw
	}
	return out + "…"
}

func join(gs []string) string {
	s := ""
	for _, g := range gs {
		s += g
	}
	return s
}

func main() {
	if err := plugin.Serve(context.Background(), &card{}); err != nil {
		fmt.Fprintln(os.Stderr, "autodoc-doc-stats:", err)
		os.Exit(1)
	}
}
