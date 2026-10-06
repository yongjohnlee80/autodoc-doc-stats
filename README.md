# autodoc-doc-stats

A card beside the page in [AutoDoc](https://github.com/yongjohnlee80/autodoc) that measures the
note you are editing, as you edit it:

- words, characters, lines and sections (Markdown headings);
- the reading time, at 230 words a minute;
- the words the note uses most, without the stop words ("the", "of", "don't", …).

Select some text and the card measures the selection instead. It says which of the two it shows.

```
 notes/gardens.md
 the whole note

 Words                       1,284
 Characters                  7,301
 Lines                         112
 Sections                        9
 Reading                     6 min

Most frequent
 soil                           24
 compost                        17
 beds                           11
```

It is also an example of an AutoDoc plugin of protocol 2. It is a **card** (`[dialog] modal = false`)
that reads the **document feed** (`[feed] document = true`) and has a **command**. AutoDoc's
[plugin reference](https://github.com/yongjohnlee80/autodoc/blob/main/docs/plugins.md) describes
all three.

## Install

It needs AutoDoc v0.1.21 or newer, git, and Go 1.25 or newer, since the build is `go build`.

In AutoDoc (`autodoc --ui`): **Plugins › Add from a git URL…**, then

```
https://github.com/yongjohnlee80/autodoc-doc-stats
```

AutoDoc clones it, then asks before anything runs. The question shows the build, the command it would
start, and what the plugin declares: "a card beside the page · reads the open note's text (feed) ·
commands: Show / hide the stats (SPC p s)". It is at your own risk, as every plugin is: a plugin runs
as you. On **Yes, at my own risk**, it builds the plugin and lists **Plugins › Doc stats**.
**Plugins › Manage plugins…** updates it or removes it.

By hand, the same thing:

```sh
git clone https://github.com/yongjohnlee80/autodoc-doc-stats ~/.config/autodoc/plugins/doc-stats
cd ~/.config/autodoc/plugins/doc-stats && go build -o bin/autodoc-doc-stats .
```

## Use

| | |
|---|---|
| SPC p s | show the card, focused, or hide it |
| Plugins › Doc stats › Open, Close, or Show / hide the stats | the same, from the menu |
| a click on the card | focus it |
| ↑ ↓ (or k, j), PageUp, PageDown, Home, End | scroll the most frequent words, while focused |
| Esc | give the keys back to the note. The card stays open |

The card sits on the right of the page. **Plugins › Manage plugins… › Place** moves it to the left.
The preference `tui.plugin.doc-stats.key.toggle` changes the letter, and `""` unbinds it.

## What it counts

- **Front matter is not counted.** A YAML block that opens a note with `---` and closes with `---`
  is the note's metadata.
- **A character is what shows as one**, as the editor's cursor counts it. For example, an `é` written
  as `e` and a combining accent is one character, and so is a flag. Line breaks are not counted.
- **A word** is a run of letters and digits. An apostrophe or hyphen inside one keeps it whole
  (`don't`, `x-ray`), and `don’t` is the same word as `don't`. A link's target (`](…)`) and a bare
  URL are not words; the link's text is.
- **A section** is an ATX heading (`#` to `######`, then a space) outside a code fence, indented
  less than four spaces.
- **The most frequent words** leave out the stop words, words of one character, numbers, and the
  words inside code fences. Code still counts towards the words and the reading time.
- **A note over 1 MiB** reaches a plugin without its text, and the card says it is too large.

## How it is made

- `stats/`: the measures, with no AutoDoc in them besides the selection's positions.
  - `Of` measures a text.
  - `Selected` cuts the selection out by the feed's positions. Lines and columns count from 1, and a
    column is a grapheme cluster, cut with golib's `tui.Graphemes`, the segmentation the editor uses.
- `main.go`: the card. `plugin.Serve` calls it on one goroutine, so it needs no lock. It is a
  `plugin.Handler` that also implements the protocol-2 interfaces `plugin.DocumentReader` (the
  feed), `plugin.Focuser` (the card's focus) and `plugin.Commander`.
  - A row is written by screen columns: a wide character (한국어, most emoji) is one cell of the frame
    and two columns on the screen.
  - A path too long for the card keeps its tail.
- The tests run the card under `plugin.ServeConn` over pipes, as the TUI runs it. They open it at
  protocol 2, send it documents, focus and keys, and read its frames.

```sh
go test -race ./...
```

## License

Apache 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
