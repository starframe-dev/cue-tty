# cue-tty

Playwright for TUI applications.

`cue-tty` — Go-библиотека + `cuetty-cli` — CLI-инструмент для автоматизации и тестирования TUI-приложений (Bubbletea, ncurses, vim, htop и др.) через PTY.

## cuetty-cli

`cuetty-cli` — аналог `playwright-cli` для терминала. Каждая команда — отдельный CLI-вызов. Сессии сохраняются между вызовами через фоновый демон.

```bash
# Запустить TUI-приложение
cuetty-cli -s demo launch ./my-app

# Посмотреть структуру экрана (или element tree, если warp-based)
cuetty-cli -s demo node

# Кликнуть по элементу
cuetty-cli -s demo click button:Add

# Прочитать экран
cuetty-cli -s demo text

# Закрыть сессию
cuetty-cli -s demo exit
```

### Установка

```bash
go get github.com/starframe-dev/cue-tty
```

`cuetty-cli` глобально доступен после сборки `./cmd/cuetty-cli`.

### Команды

| Команда | Описание |
|---------|----------|
| `launch <cmd> [args...]` | Запустить TUI-приложение |
| `text` | Показать plain text экрана |
| `lines` | Показать строки с номерами |
| `line <n>` | Показать конкретную строку |
| `cursor` | Показать позицию курсора |
| `press <key>` | Нажать клавишу |
| `type <text>` | Напечатать текст |
| `click <x> <y>` | Клик мышью по координатам |
| `click <role>:<name>` | Клик по элементу (warp /elements) |
| `scroll <up|down> <x> <y>` | Скролл |
| `mouse <down|up> <x> <y>` | Mouse events |
| `node` | Граф нод / element tree |
| `save [name]` | Сохранить артефакты |
| `resize <cols> <rows>` | Изменить размер терминала |
| `wait <ms>` | Подождать |
| `expect <contain|not> <text>` | Ассерт |
| `show` | Dashboard всех сессий |
| `list` | Список сессий |
| `exit` | Закрыть сессию |
| `daemon` | Запустить демон вручную |

## Пример: warp-demo

В `examples/warp-demo/` есть полноценный пример TUI-приложения на [warp](https://github.com/starframe-dev/warp) с HTTP `/elements` endpoint и мышью.

```bash
cd examples/warp-demo
go build -o warp-demo ./...

cuetty-cli -s demo launch ./warp-demo
cuetty-cli -s demo wait 500
cuetty-cli -s demo node
cuetty-cli -s demo click button:Add
cuetty-cli -s demo text
cuetty-cli -s demo exit
```

Тесты:

```bash
cd examples/warp-demo
go test ./... -v
```

## Go API

Если нужно писать тесты на Go напрямую, используй `pkg/cue`:

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

### API

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
# cue-tty library tests
go test ./...

# warp-demo E2E tests
cd examples/warp-demo
go test ./... -v
```
