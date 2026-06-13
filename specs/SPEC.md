# cue-tty — аналог Playwright для TUI приложений

## Контекст

Существует Playwright для веба, но для TUI-приложений нет удобного, современного инструмента автоматизации. Существующие решения (expect, tmux-based скрипты) низкоуровневые, не дают AI-видимости внутрь приложения и не генерируют выразительные артефакты.

Starframe/cue-tty создаёт Playwright-подобный API для тестирования любых TUI-приложений: Bubbletea, ncurses, vim, htop и т.д.

## Цель

Создать Go-библиотеку `cue-tty` с API в стиле Playwright для запуска, взаимодействия и тестирования TUI-приложений через PTY. MVP: launch, keyboard input, mouse input, screen capture (text), assertions.

## Что изменится

1. `go.mod` — инициализация Go-модуля `github.com/starframe-dev/cue-tty`
2. `pkg/cue/cue.go` — публичный API: `Launch()`, `Connect()`
3. `pkg/cue/app.go` — `App` (процесс + PTY), lifecycle
4. `pkg/cue/page.go` — `Page` (запросы к экрану, keyboard, mouse, scroll)
5. `pkg/cue/keyboard.go` — маппинг клавиш, `Press()`, `Type()`, mouse SGR protocol
6. `pkg/cue/expect.go` — assertions: `ToContain`, `ToMatch`, `ToHaveLine`
7. `pkg/cue/options.go` — `WithSize`, `WithArgs`, `WithEnv`, `WithTimeout`
8. `pkg/ansi/parser.go` — парсер ANSI escape-последовательностей
9. `pkg/ansi/screen.go` — виртуальный экран (flat cell slice для скорости)
10. `pkg/ansi/style.go` — стили ячеек (цвета, bold, etc)
11. `pkg/cue/artifact.go` — генерация артефактов (render.txt, state.json, timeline.md)
12. Тесты на каждый пакет

## Детали реализации

### 1. PTY Backend (`pkg/pty/`)

Используем `github.com/creack/pty`.

```go
type Backend struct {
    master *os.File
    cmd    *exec.Cmd
}

func Start(command string, args []string, env []string, dir string, cols, rows int) (*Backend, error)
func (b *Backend) Read(p []byte) (int, error)
func (b *Backend) Write(p []byte) (int, error)
func (b *Backend) Resize(cols, rows int) error
func (b *Backend) Close() error
```

### 2. ANSI Parser (`pkg/ansi/`)

Парсим ANSI escape-последовательности в виртуальный экран.

Поддерживаем:
- Cursor movement: CSI n A/B/C/D, CSI H (CUP), CSI J (ED), CSI K (EL)
- Colors: SGR 30-37, 38;5;n, 38;2;r;g;b (и фон 40-47, 48;5;n, 48;2;r;g;b)
- Styles: bold, dim, italic, underline, reverse
- Alternate screen buffer: `ESC [ ? 1049 h/l`
- Scroll regions

Screen использует **flat slice** `[rows*cols]Cell` вместо `[][]Cell` — cache-friendly, 1 аллок вместо rows+1.

```go
type Screen struct {
    cols, rows int
    cells      []Cell      // flat: cells[row*cols + col]
    altCells   []Cell
    cursor     Cursor
    altCursor  Cursor
    // ...
}

type Cell struct {
    Rune  rune
    Style Style
}
```

### 3. Page API (`pkg/cue/page.go`)

```go
type Page struct {
    app *App
}

func (p *Page) Press(key string) error           // "Enter", "Ctrl+C", "F1", "Up"
func (p *Page) Type(text string) error           // посимвольный ввод
func (p *Page) Text() (string, error)            // plain text без ANSI
func (p *Page) Lines() ([]string, error)         // построчно
func (p *Page) Line(n int) (string, error)       // конкретная строка (0-based)
func (p *Page) Cursor() (row, col int, err error)
func (p *Page) MouseClick(x, y int) error       // click at cell (x,y)
func (p *Page) MouseScrollUp(x, y int) error    // wheel up
func (p *Page) MouseScrollDown(x, y int) error  // wheel down
func (p *Page) MouseDown(button, x, y int) error
func (p *Page) MouseUp(button, x, y int) error
func (p *Page) EnableMouse() error               // ESC[?1000h ESC[?1006h
func (p *Page) DisableMouse() error              // ESC[?1000l ESC[?1006l
```

### 4. Keyboard + Mouse Mapping (`pkg/cue/keyboard.go`)

```go
var keyMap = map[string][]byte{
    "Enter":     []byte{0x0D},
    "Up":        []byte{0x1B, 0x5B, 0x41},
    // ... F1-F12
}

func ResolveKey(key string) []byte
func MouseSGR(button, x, y int, release bool) []byte  // SGR extended protocol
```

### 5. Expect API (`pkg/cue/expect.go`)

```go
func Expect(p *Page) *Expectation

func (e *Expectation) ToContain(substr string) error
func (e *Expectation) ToMatch(pattern string) error  // regex
func (e *Expectation) ToHaveLine(row int, expected string) error
func (e *Expectation) ToHaveCursorAt(row, col int) error
func (e *Expectation) ToNotContain(substr string) error
```

### 6. Artifacts (`pkg/cue/artifact.go`)

```go
type Artifact struct {
    Name      string
    Render    string
    Timeline  []Event
}

func (p *Page) SaveArtifact(dir, name string) error  // render.txt + state.json
func WriteTimeline(dir string, events []Event) error
```

### 7. Launch Options (`pkg/cue/options.go`)

```go
type Options struct {
    Args    []string
    Env     []string
    Dir     string
    Width   int  // cols
    Height  int  // rows
    Timeout time.Duration
}
```

## Критерии приёмки

- [ ] `go test ./...` проходит без ошибок
- [ ] `go vet ./...` чист
- [ ] Можно запустить `echo hello`, увидеть текст на экране
- [ ] `expect(page).ToContain("hello")` работает
- [ ] Мышь: MouseClick, MouseScrollUp/Down генерируют корректные SGR-последовательности
- [ ] Артефакты (render.txt, state.json) генерируются
- [ ] README с примером использования
- [ ] Flat cell slice для производительности (вместо [][]Cell)
