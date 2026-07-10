# cue-tty CLI — интерактивный REPL для тестирования TUI

## Контекст

Сейчас cue-tty — только Go-библиотека. Чтобы тестировать TUI-приложения, нужно писать Go-код. Нет способа быстро запустить приложение, поэкспериментировать с ним в интерактивном режиме, посмотреть screen state, нажать клавиши и увидеть результат.

Playwright предоставляет `playwright cli` для интерактивной работы с браузером. Нам нужно то же самое для TUI.

## Цель

Создать CLI-инструмент `cuetty-cli` с:
- Флагом `-s <session_name>` для именованных сессий (чтобы можно было запускать несколько параллельно)
- REPL-режимом с командами: launch, press, type, text, lines, cursor, click, scroll, render, node, save, exit
- Возможностью смотреть "граф нод" — визуализацию screen state (структуру экрана с ячейками, стилями, курсором)

## Что изменится

1. `cmd/cuetty-cli/main.go` — точка входа, парсинг флагов, запуск REPL
2. `pkg/cli/repl.go` — REPL-цикл: чтение команд, диспатч, вывод результата
3. `pkg/cli/commands.go` — обработчики команд (launch, press, type, text, lines, cursor, click, scroll, render, node, save, exit)
4. `pkg/cli/session.go` — управление сессиями (именованные сессии через `-s`)
5. `pkg/cli/nodes.go` — "граф нод": визуализация screen state (структура экрана, ячейки, стили, курсор)

## Детали реализации

### 1. Точка входа (`cmd/cuetty-cli/main.go`)

```go
func main() {
    sessionName := flag.String("s", "default", "session name")
    flag.Parse()

    cli.Run(*sessionName)
}
```

### 2. REPL (`pkg/cli/repl.go`)

Интерактивный цикл с приглашением `cue-tty> `.

Команды:
- `launch <command> [args...]` — запустить TUI-приложение
- `press <key>` — нажать клавишу (Enter, Ctrl+C, F1, Up, a, etc.)
- `type <text>` — напечатать текст
- `text` — показать plain text экрана
- `lines` — показать все строки с номерами
- `line <n>` — показать конкретную строку
- `cursor` — показать позицию курсора
- `click <x> <y>` — клик мышью
- `scroll up <x> <y>` / `scroll down <x> <y>` — скролл
- `mouse down/up <button> <x> <y>` — mouse events
- `render` — показать ANSI-рендер экрана (с цветами)
- `node` — показать "граф нод" (структуру screen)
- `save <name>` — сохранить артефакт
- `resize <cols> <rows>` — изменить размер терминала
- `wait <ms>` — подождать
- `expect contain <text>` — проверить, что текст есть на экране
- `exit` / `quit` / `Ctrl+D` — выход

### 3. Сессии (`pkg/cli/session.go`)

Именованные сессии позволяют запускать несколько независимых TUI-приложений параллельно.

```go
type Session struct {
    Name string
    App  *cue.App
    Page *cue.Page
}

type SessionManager struct {
    mu       sync.Mutex
    sessions map[string]*Session
}
```

### 4. Граф нод (`pkg/cli/nodes.go`)

Визуализация screen state в виде дерева/таблицы:

```
Screen 80x24
├── Cursor: (0, 0)
├── ScrollRegion: 0-23
├── AltBuffer: false
├── Style: fg=#e5e5e5 bg=#000000
├── Lines:
│   ├── [0] "~                                         "
│   ├── [1] "~                                         "
│   ├── [2] "~                                         "
│   └── ...
└── Cells (1920 total):
    ├── [0,0] '~' fg=#e5e5e5 bg=#000000
    ├── [0,1] ' ' fg=#e5e5e5 bg=#000000
    └── ...
```

### 5. ANSI-рендер (`render`)

Показывает экран с ANSI-цветами в терминале (используя текущие ANSI escape codes для подсветки).

### 6. Обработка ошибок

- Если приложение не запущено, команды press/type/text/etc. возвращают ошибку "no app running"
- Если приложение уже запущено, `launch` возвращает ошибку "app already running, use exit first"
- `exit` закрывает приложение и завершает REPL

## Критерии приёмки

- [ ] `go build ./cmd/cuetty-cli` собирается без ошибок
- [ ] `cuetty-cli -s test` запускает REPL с приглашением `cue-tty> `
- [ ] `launch echo hello` запускает echo, `text` показывает "hello"
- [ ] `press Enter`, `type "text"` работают
- [ ] `cursor` показывает позицию курсора
- [ ] `click 10 5` отправляет mouse event
- [ ] `render` показывает ANSI-рендер
- [ ] `node` показывает граф нод
- [ ] `save test` сохраняет render.txt и state.json
- [ ] `exit` завершает REPL
- [ ] `go vet ./...` чист
