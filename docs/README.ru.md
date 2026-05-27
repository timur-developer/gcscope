# gcviz

![gcviz demo](assets/demo_hero.gif)

Read this in other languages: [English](../README.md)

`gcviz` — TUI-инструмент для live-наблюдения за поведением Go GC: GC-циклы, STW-паузы, динамика heap и pacing прямо в терминале.

Основной режим — `run`: `gcviz` сам запускает ваш Go-бинарь с `GODEBUG=gctrace=1,gcpacertrace=1` и парсит stderr. Без изменений кода приложения.

## Быстрый старт

![gcviz launch](assets/demo_launch.gif)

Требования: Go 1.25+, достаточно большой терминал.

Запуск встроенной демо-нагрузки:

```bash
go run ./cmd/gcviz lab churn
```

Или через Makefile:

```bash
make lab-churn
```

## Режимы

### run (основной)

```bash
gcviz run ./path/to/your-binary -- --your-flag value
```

Из исходников (без установки):

```bash
go run ./cmd/gcviz run ./path/to/your-binary -- --your-flag value
```

### lab

Встроенные пресеты:

```bash
gcviz lab alloc
gcviz lab churn
gcviz lab idle
gcviz lab spike
```

### attach (вторичный)

Подключение к уже работающему сервису, который экспортирует `runtime/metrics` в JSON-формате, понятном `gcviz`.

1. Добавьте `pkg/reporter` в сервис:

```go
package main

import (
	"log"
	"net/http"

	"github.com/timur-developer/gcviz/pkg/reporter"
)

func main() {
	rep := reporter.New()
	mux := http.NewServeMux()
	mux.Handle(rep.Path(), rep.Handler())
	log.Fatal(http.ListenAndServe(":8080", mux))
}
```

2. Подключитесь:

```bash
gcviz attach http://127.0.0.1:8080/gcviz/metrics
```

### diff

Сравнение двух snapshot-файлов:

```bash
gcviz diff ./a.json ./b.json
```

## Управление

![gcviz features](assets/demo_features.gif)

База:

- `?` / `h` / `f1` показать/скрыть help
- `q` выход
- `space` пауза/продолжить обновление
- `left` / `right` листать историю (в паузе)
- `s` сохранить snapshot

Интерфейс:

- `g` переключить layout (spaced/tight)
- `l` переключить режим подписей STW на bar chart

Графики:

- `z` выбрать активный график (Heap/STW)
- `+` / `-` Y-zoom для активного графика
- `0` сброс Y-zoom/pan активного графика
- `[` / `]` X-zoom (масштаб времени)
- `shift+up` / `shift+down` Y-pan активного графика
- `r` полный сброс масштаба/панорамирования

## Настройки

Глобальные флаги (и env):

- `--window-size` (`GCVIZ_WINDOW_SIZE`) (default: 200)
- `--snapshot-path` (`GCVIZ_SNAPSHOT_PATH`) (default: `tmp/snapshots`)
- `--exit-snapshot` (`GCVIZ_EXIT_SNAPSHOT`) (default: true)
- `--no-alt-screen` (`GCVIZ_NO_ALT_SCREEN`)
- `--stw-warn-us` (`GCVIZ_STW_WARN_US`) (default: 200)
- `--stw-bad-us` (`GCVIZ_STW_BAD_US`) (default: 1000)

Режимные env:

- `GCVIZ_RUN_TARGET`
- `GCVIZ_ATTACH_URL`, `GCVIZ_POLL_INTERVAL`
- `GCVIZ_LAB_PRESET`
- `GCVIZ_DIFF_A`, `GCVIZ_DIFF_B`

## Snapshots

- Директория по умолчанию: `tmp/snapshots`
- Ручной snapshot: `s`
- Snapshot на выходе включён по умолчанию; пропускается, если недавно делали ручной

## Notes / FAQ

- В `attach` режиме нельзя узнать env целевого процесса (`GOGC`, `GOMEMLIMIT`, `GODEBUG`), поэтому UI показывает `n/a`.
- Очень маленькие STW могут отображаться как 0 из-за форматирования `gctrace`.

## Для разработки

```bash
make ci
make lint
make test
make build
```

## License

MIT. См. [LICENSE](../LICENSE).

