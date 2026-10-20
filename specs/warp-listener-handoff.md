# Устранение гонки Warp HTTP-порта через передачу listener

## Контекст
`Session.Launch` выбирает свободный TCP-порт, закрывает listener и только затем запускает дочерний Warp-процесс с `WARP_HTTP_PORT`. Между закрытием и bind в Warp остаётся TOCTOU-окно: другой процесс может занять порт. Bearer token предотвращает тихое подключение к чужому сервису, но не гарантирует успешный запуск.

## Цель
Передать уже открытый listener от cue-tty дочернему Warp-приложению через наследуемый файловый дескриптор. Порт остаётся занят тем же сокетом до начала `http.Server.Serve`, поэтому промежуточного освобождения/повторного bind нет.

## Что изменится
1. Репозиторий `starframe-dev/warp`: добавить публичный метод запуска HTTP inspector на переданном `net.Listener`, сохранив существующие `ServeHTTP`/`ServeHTTPWithOptions`.
2. Репозиторий `starframe-dev/cue-tty`: добавить передачу дополнительных файлов в PTY child (`exec.Cmd.ExtraFiles`), создавать listener на `127.0.0.1:0`, передавать его дочернему процессу и закрывать родительские копии после старта.
3. `examples/warp-demo`: принимать listener FD из env и использовать новый Warp API; сохранить текущий запуск без FD для standalone use.
4. Тесты обоих репозиториев: владение listener, HTTP inspector на переданном сокете, наследование FD через PTY, реальный `/elements` и отсутствие повторного выбора порта.
5. Обновить Warp dependency в cue-tty до нового опубликованного patch release, workflow/документацию; запустить матрицу и E2E.

## Детали реализации
- Warp API принимает `net.Listener`; после успешной передачи владение listener переходит Warp HTTP server. При ошибке до запуска server вызывающая сторона остаётся ответственной за закрытие.
- cue-tty создаёт TCP listener до старта дочернего процесса, берёт дубликат `*os.File`, передаёт его через `exec.Cmd.ExtraFiles` на fd 3 и устанавливает `WARP_HTTP_FD=3` вместе со случайным bearer token.
- Родитель закрывает listener и дубликат только после возврата успешного PTY start; дочерний Warp процесс импортирует listener из fd 3 через `net.FileListener` и передаёт его Warp API.
- Если запуск дочернего процесса завершается ошибкой, cue-tty закрывает все родительские ресурсы. Старый `WARP_HTTP_PORT` путь остаётся fallback для внешних приложений, которые его используют.
- Изменения публикуются как Warp patch release `v0.1.2`, затем cue-tty фиксирует эту версию.

## Критерии приёмки
- [x] Warp HTTP server обслуживает HTTP на переданном listener без собственного bind; тесты подтверждают ownership и `CloseHTTP`.
- [x] cue-tty передаёт listener FD дочернему процессу; PTY test проверяет fd 3, реальный E2E обращается к `/elements` через этот listener.
- [x] Ошибки запуска закрывают parent listener/file через defer; Warp shutdown закрывает listener; тесты покрывают оба пути.
- [x] Warp и cue-tty tidy/build/test/race/vet/govulncheck проходят локально; Warp E2E проходит на опубликованном v0.1.2.
- [x] Warp `v0.1.2` опубликован. Warp hosted CI для Ubuntu/macOS/Windows и Go 1.22/1.26/1.27 зелёный.
- [ ] cue-tty изменения отправлены; hosted Linux/macOS CI cue-tty зелёный.
