package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yongjohnlee80/autodoc/plugin"
	"github.com/yongjohnlee80/golib/tui"
)

// host is a test's end of the link: what AutoDoc's TUI would be.
type host struct {
	link   *plugin.Link
	frames chan []plugin.Row
	ready  chan int
}

// start runs the card under plugin.ServeConn over two pipes, as the TUI runs a plugin, and opens it
// at protocol 2, w by h.
func start(t *testing.T, w, h int) *host {
	t.Helper()
	toPlugin, fromHost, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	fromPlugin, toHost, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = plugin.ServeConn(ctx, plugin.FileConn(toPlugin, toHost), &card{}) }()
	hs := &host{frames: make(chan []plugin.Row, 256), ready: make(chan int, 1)}
	hs.link, err = plugin.NewLink(ctx, plugin.FileConn(fromPlugin, fromHost), func(m string, p []any) {
		switch m {
		case plugin.MethodFrame:
			if rows, _, err := plugin.ReadFrame(p); err == nil {
				hs.frames <- rows
			}
		case plugin.MethodReady:
			if v, err := plugin.ReadReady(p); err == nil {
				hs.ready <- v
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { hs.link.Close() })
	hs.send(t, plugin.MethodOpen, plugin.OpenParams(plugin.Open{Protocol: 2, Width: w, Height: h,
		Theme: plugin.Theme{Name: "dark", Colors: map[string]string{"document.cursor": "#ffaf00"}}}))
	select {
	case v := <-hs.ready:
		if v != 2 {
			t.Fatalf("answered protocol %d, want 2", v)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no host.ready")
	}
	return hs
}

func (hs *host) send(t *testing.T, method string, params []any) {
	t.Helper()
	if err := hs.link.Notify(context.Background(), method, params); err != nil {
		t.Fatalf("%s: %v", method, err)
	}
}

// until returns the first frame whose text has every one of want, failing after 5 s.
func (hs *host) until(t *testing.T, want ...string) []plugin.Row {
	t.Helper()
	deadline := time.After(5 * time.Second)
	last := ""
	for {
		select {
		case rows := <-hs.frames:
			last = text(rows)
			ok := true
			for _, w := range want {
				ok = ok && strings.Contains(last, w)
			}
			if ok {
				return rows
			}
		case <-deadline:
			t.Fatalf("no frame with %q; the last:\n%s", want, last)
		}
	}
}

func text(rows []plugin.Row) string {
	var b strings.Builder
	for _, row := range rows {
		b.WriteString(rowText(row))
		b.WriteByte('\n')
	}
	return b.String()
}

func rowText(row plugin.Row) string {
	s := ""
	for _, r := range row {
		s += r.Text
	}
	return s
}

// lineWith is the first row holding s.
func lineWith(t *testing.T, rows []plugin.Row, s string) string {
	t.Helper()
	for _, r := range rows {
		if l := rowText(r); strings.Contains(l, s) {
			return l
		}
	}
	t.Fatalf("no row with %q", s)
	return ""
}

func doc(text string, version int, sel ...plugin.Range) []any {
	return plugin.DocumentParams(plugin.Document{Path: "notes/gardens.md", Workspace: "kb", Text: text,
		Cursor: plugin.Position{Line: 1, Col: 1}, Selection: sel, Version: version})
}

// TestTheCardMeasuresTheNoteThenTheSelection: the feed's note is measured whole; a selection is
// measured instead, and says so; the selection gone, the whole note again.
func TestTheCardMeasuresTheNoteThenTheSelection(t *testing.T) {
	hs := start(t, 34, 20)
	hs.until(t, "Waiting for the note")
	note := "# Gardens\n\nsoil soil compost\n\n## Beds\n\nraised beds of soil"
	hs.send(t, plugin.MethodDocument, doc(note, 1))
	rows := hs.until(t, "notes/gardens.md", "the whole note")
	for label, want := range map[string]string{"Words": "9", "Lines": "7", "Sections": "2", "Reading": "1 min"} {
		if l := lineWith(t, rows, label); !strings.HasSuffix(strings.TrimRight(l, " "), want) {
			t.Errorf("%s row %q: want it to end in %s", label, l, want)
		}
	}
	if l := lineWith(t, rows, "soil"); !strings.HasSuffix(strings.TrimRight(l, " "), "3") {
		t.Errorf("soil row %q: want 3", l)
	}

	sel := plugin.Range{Start: plugin.Position{Line: 7, Col: 1}, End: plugin.Position{Line: 7, Col: 12}}
	hs.send(t, plugin.MethodDocument, doc(note, 1, sel)) // "raised beds"
	rows = hs.until(t, "the selection")
	if l := lineWith(t, rows, "Words"); !strings.HasSuffix(strings.TrimRight(l, " "), "2") {
		t.Errorf("selected words row %q: want 2", l)
	}

	hs.send(t, plugin.MethodDocument, doc(note, 2))
	hs.until(t, "the whole note")
}

// TestTooLargeAndNoNoteSayWhy: a note over the link's limit comes with no text; no note open has no
// path and no text.
func TestTooLargeAndNoNoteSayWhy(t *testing.T) {
	hs := start(t, 40, 12)
	hs.send(t, plugin.MethodDocument, plugin.DocumentParams(plugin.Document{Path: "big.md", Version: 3, TooLarge: true}))
	hs.until(t, "big.md", "Too large to measure")
	hs.send(t, plugin.MethodDocument, plugin.DocumentParams(plugin.Document{Version: 4}))
	hs.until(t, "No note open.")
	hs.send(t, plugin.MethodDocument, plugin.DocumentParams(plugin.Document{Text: "draft words", Version: 5}))
	hs.until(t, "a draft, not saved")
}

// TestFocusedKeysScrollTheFrequentWords: focused, the card says how to scroll and give the keys
// back; Down scrolls the list by one; unfocused, the hint goes.
func TestFocusedKeysScrollTheFrequentWords(t *testing.T) {
	hs := start(t, 30, 12) // 12 rows: 2 of words while focused
	hs.send(t, plugin.MethodDocument, doc("alpha alpha alpha beta beta gamma", 1))
	hs.until(t, "alpha", "beta")
	hs.send(t, plugin.MethodFocus, plugin.FocusParams(true))
	rows := hs.until(t, "Esc back")
	if strings.Contains(text(rows), "gamma") {
		t.Fatalf("gamma shows before a scroll, with 2 rows:\n%s", text(rows))
	}
	hs.send(t, plugin.MethodKey, plugin.KeyParams(plugin.Key{Key: "Down"}))
	rows = hs.until(t, "gamma")
	if strings.Contains(text(rows), "alpha") {
		t.Errorf("alpha still shows after Down:\n%s", text(rows))
	}
	hs.send(t, plugin.MethodKey, plugin.KeyParams(plugin.Key{Key: "End"}))
	hs.send(t, plugin.MethodKey, plugin.KeyParams(plugin.Key{Key: "Home"}))
	hs.until(t, "alpha")
	hs.send(t, plugin.MethodFocus, plugin.FocusParams(false))
	rows = hs.until(t, "gamma") // 3 rows unfocused
	if strings.Contains(text(rows), "Esc back") {
		t.Errorf("the hint stays unfocused:\n%s", text(rows))
	}
}

// TestCountsMeetTheRightEdgeOnScreen: a wide word (한국어: six columns in three cells) still puts
// its count one column in from the card's right edge, as the screen shows it, not as the cells
// count; and a path too long for the card keeps its tail.
func TestCountsMeetTheRightEdgeOnScreen(t *testing.T) {
	hs := start(t, 24, 14)
	hs.send(t, plugin.MethodDocument, plugin.DocumentParams(plugin.Document{
		Path: "a/very/long/directory/name/of/notes/gardens.md", Text: "한국어 한국어 plain plain", Version: 1}))
	rows := hs.until(t, "한국어", "plain")
	for _, w := range []string{"한국어", "plain"} {
		l := lineWith(t, rows, w)
		end := strings.TrimRight(l, " ")
		if got := tui.StringWidth(end); got != 23 {
			t.Errorf("%q: its count ends at column %d on screen, want 23 (one in from 24)", l, got)
		}
	}
	if l := rowText(rows[0]); !strings.HasPrefix(l, "…") || !strings.Contains(l, "gardens.md") ||
		tui.StringWidth(strings.TrimRight(l, " ")) > 24 {
		t.Errorf("path row %q: want …, the tail, and no wider than 24", l)
	}
}

func TestThousands(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 1234567: "1,234,567"} {
		if got := thousands(n); got != want {
			t.Errorf("thousands(%d) = %q, want %q", n, got, want)
		}
	}
}
