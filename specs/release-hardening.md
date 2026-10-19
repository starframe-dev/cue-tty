# Cue-TTY release hardening

## Контекст
На audited commit `96e341d485ee2e6ea481d92bf175080c9c4aaa5d` базовые проверки Linux/macOS проходят, но CI останавливается на `govulncheck`: проект и workflow используют Go 1.26.8 при известных исправлениях в 1.26.9. Аудит также выявил конкурентное чтение Session, блокирующую PTY-запись, неполную проверку Resize, несогласованные HTTP/командные таймауты и недочёты Mouse SGR/NodeTree.

## Цель
Обновить минимальную версию Go до безопасной поддерживаемой версии и устранить подтверждённые P1/P2 дефекты без переписывания архитектуры. Сопроводить каждый исправленный сценарий тестом и выполнить root/Warp проверочную матрицу.

## Что изменится
1. `go.mod`, `go.sum`, `.github/workflows/go.yml` — безопасная версия Go и зависимости.
2. `pkg/cli/session.go`, `pkg/cli/commands.go`, `pkg/cli/daemon.go` — синхронизированные снимки сессий, отмена команд и согласованные deadlines.
3. `pkg/cue/app.go`, `pkg/pty/pty.go` — отменяемая сериализованная PTY-запись и атомарно проверяемый resize.
4. `pkg/cue/keyboard.go`, `pkg/cue/page.go` — корректные SGR releases, drag и документированный mouse API.
5. `pkg/cli/nodes.go` и связанные ANSI/Screen API — размеры и Unicode ширина/ячейки из согласованного screen snapshot.
6. Тесты CLI/PTY/App/ANSI/Warp и точечные README/spec обновления — регрессии и проверяемый контракт.

## Детали реализации
- Сначала воспроизвести govulncheck findings на текущем toolchain, затем поднять `go` directive и CI минимум до 1.26.9 (или выше, если актуальный сканер требует).
- Для SessionManager не удерживать manager lock при взятии session lock; добавить методы состояния/snapshot и использовать per-session serialization.
- Ввести контекстную PTY write path с deadline/cancellation и корректной обработкой partial writes; закрытие должно разблокировать запись и завершиться без утечки reader/writer goroutines.
- Валидировать `App.Resize` до ioctl/screen mutation: положительные размеры, границы uint16 и документированный верхний предел; не менять ни одну сторону при ошибке.
- Клиентский HTTP timeout должен превышать максимальную длительность допустимой команды; серверные write/read deadlines должны допускать этот интервал. Прерывать команду по request context и ограничивать durations без overflow.
- Кодировать SGR button release реальным button id; проверять кнопки, drag state и coordinate bounds.
- NodeTree должен брать размер, cursor, строки и cells одним snapshot; display width считать Unicode grapheme/cell widths (wide + combining), не byte length.
- Удалять только фиксированные sleeps, для которых можно ждать подтверждаемое состояние; оставить необходимые protocol/debounce delays с обоснованием.

## Результаты производительности
Замер на Apple M4 / Go 1.27.2 до hardening → после него; разница мала и укладывается в шум короткого бенчмарка:

| Benchmark | До | После | Δ |
|---|---:|---:|---:|
| StreamOutput 80×24 | 60.844 µs/op | 59.073 µs/op | −2.9% |
| StreamOutput 200×60 | 310.168 µs/op | 308.881 µs/op | −0.4% |
| SnapshotRender 80×24 | 38.844 µs/op | 40.113 µs/op | +3.3% |
| SnapshotRender 200×60 | 331.610 µs/op | 339.496 µs/op | +2.4% |

Текущие `B/op`: StreamOutput 8,071/25,315; SnapshotRender 387,580/2,447,907. Существенной регрессии не видно.

## Остаточные блокеры
- `Session.Launch` освобождает выбранный ephemeral port перед bind HTTP server в Warp child. Bearer token обнаруживает чужой endpoint, но не предотвращает редкую port-allocation race и ошибку старта; надёжное решение требует listener handoff/поддержки в Warp.
- GitHub Actions matrix добавлен, но hosted Linux/macOS run для этого изменения не запускался.

## Критерии приёмки
- [x] Root tidy/build/test/race/vet и Warp tidy/build/test/race/vet проходят на Go 1.26.9.
- [x] govulncheck root и Warp не находят достижимых уязвимостей; найденный module-only GO-2026-5024 в Warp `x/sys` исправлен обновлением до v0.44.0. `x/text` v0.3.8 не фигурирует в достижимом scan graph.
- [x] Тесты concurrent show/list/launch/close и race проходят.
- [x] Заблокированная PTY-запись отменяется закрытием/выходом, partial writes сериализованы; reader/writer и child processes очищаются.
- [x] Resize отклоняет invalid/overflow/oversized размеры без изменения PTY/Screen и корректно сериализуется.
- [x] Большие допустимые wait/waitfor deadline работают; отрицательные/overflow значения отклоняются; HTTP budgets согласованы.
- [x] Mouse SGR releases, drag state, допустимость кнопок и координаты покрыты тестами.
- [x] NodeTree размеры/ячейки/курсор берутся одним snapshot; wide и combining graphemes и styles проверены.
- [x] Полный Warp E2E проходит локально на macOS; workflow покрывает Linux/macOS; Linux root и Warp cross-builds прошли.
- [x] Benchmarks выполнены; локальные результаты сверены с прежним M4 baseline.
- [ ] Hosted Linux/macOS Actions CI зелёный (workflow обновлён, но в этом запуске hosted workflow не запускался).
- [ ] Итоговый статус READY выставляется только после подтверждения всех критериев и hosted CI.
