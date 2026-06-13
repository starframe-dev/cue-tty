# cue-tty

Playwright for TUI applications.

```go
app, err := cue.Launch("vim",
    cue.WithArgs("test.txt"),
    cue.WithSize(80, 24),
)
if err != nil {
    log.Fatal(err)
}
defer app.Close()

page := app.Page()
page.WaitStable(500 * time.Millisecond)

page.Press("i")
page.Type("hello world")
page.Press("Escape")
page.Type(":wq")
page.Press("Enter")

err = cue.Expect(page).ToContain("hello world")
```

## Install

```bash
go get github.com/starframe-dev/cue-tty
```

## API

| Method | Description |
|--------|-------------|
| `cue.Launch(cmd, opts...)` | Start a TUI app in a PTY |
| `page.Press("Enter")` | Send a key |
| `page.Type("text")` | Type text |
| `page.Text()` | Get plain screen text |
| `page.Lines()` | Get screen as lines |
| `page.MouseClick(x, y)` | Click a cell |
| `page.MouseScrollUp(x, y)` | Scroll wheel up |
| `page.MouseScrollDown(x, y)` | Scroll wheel down |
| `page.EnableMouse()` | Enable mouse tracking |
| `cue.Expect(page).ToContain("x")` | Assert text exists |
| `cue.Expect(page).ToMatch("re")` | Assert regex match |
| `cue.Expect(page).ToHaveLine(0, "x")` | Assert line content |
| `page.SaveArtifact("dir", "name")` | Save render.txt + state.json |

## Run tests

```bash
go test ./...
```
