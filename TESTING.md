# Тестирование проекта

## Что и где лежит

| Пакет | Файлы | Что проверяет |
|---|---|---|
| `dns-collector/internal/config` | `config_test.go` | загрузка YAML, env-overrides, defaults, валидация |
| `dns-collector/internal/database` | `database_test.go` | SQL-конструкции и часть сценариев (sqlmock) |
| `dns-collector/internal/server` | `udp_test.go` | JSON-парсинг, `trimInvalidJSONSuffix`, структуры |
| `dns-collector/internal/resolver` | `resolver_test.go` | конфигурация, CNAME, часть SQL |
| `dns-collector/internal/cleanup` | `cleanup_test.go` | сервис cleanup |
| `dns-collector/internal/metrics` | `*_test.go` | реестр, InfluxDB, DB-коллектор |
| `web-api/internal/handlers` | `handlers_test.go` | HTTP handlers через mock `database.DB` |
| `web-api/internal/database` | `database_test.go` | валидация regex, без реальной БД |
| `web-api/internal/excel` | `exporter_test.go` | генерация XLSX, стили, большие выборки |
| `web-api/internal/utils` | `ip_parser_test.go` | парсинг IP из файла |
| `web-api/cmd` | `main_test.go` | загрузка/валидация конфига |

## Запуск

### Корневой Makefile

~~~bash
make test                 # оба сервиса
make test-dns-collector
make test-web-api
make test-coverage
make test-verbose
~~~

### Напрямую

~~~bash
# dns-collector
cd dns-collector
go test ./internal/... ./cmd/...
go test -race ./internal/...
go test -cover ./internal/...

# web-api
cd web-api
go test ./internal/... ./cmd/...
go test -race ./internal/...
go test -cover ./internal/... ./cmd/...
~~~

### CI

Процесс ветвления, PR и релиза — [CONTRIBUTING.md](CONTRIBUTING.md).

`.github/workflows/test.yml` запускает:

- `go test -v -race -coverprofile ...` для обоих сервисов;
- `golangci-lint`;
- Codecov.

`.github/workflows/build.yml` дополнительно собирает Docker-образы, но содержит `continue-on-error: true` для тестов и версию Go 1.21, что расходится с `go.mod` (1.24). Это исправляется в [плане](docs/OPTIMIZATION_PLAN.md#этап-9-тесты-ci-наблюдаемость-34-дня).

## Покрытие (замер 2026-11-22)

| Пакет | Покрытие |
|---|---:|
| collector/config | 91,8% |
| collector/database | 42,5% |
| collector/metrics | 64,2% |
| collector/cleanup | 2,1% |
| collector/resolver | 7,1% |
| collector/server | 8,7% |
| web-api/utils | 94,1% |
| web-api/excel | 75,8% |
| web-api/handlers | 39,9% |
| web-api/metrics | 39,8% |
| web-api/cmd | 31,6% |
| web-api/database | 2,6% |

Все тесты проходят, в том числе `-race`. Низкое покрытие — не потому, что тестов нет, а потому что они проверяют конфиги/парсинг, а не реальные SQL и бизнес-логику.

## Известные пробелы

- `handleMessage` в `udp.go` не тестируется: сервер принимает конкретный `*database.Database`, а не интерфейс.
- `runResolution`, `resolveDomain`, `ApplyResolution` (после рефакторинга) не покрыты.
- SQL-запросы в `web-api/internal/database` проверяются только на валидацию regex; реальные запросы не выполняются.
- Моки в `resolver_test.go` и `udp_test.go` объявлены, но не используются (`_ = mockDB`) и имеют устаревшие сигнатуры.
- Frontend (Vue) не имеет тестов и линтера.
- `web-api/Makefile` вызывает `npm run test`, которого нет в `package.json`.
- Нет нагрузочных тестов на поток UDP и экспорт 100 тыс. строк.

## Как писать тесты

### Table-driven

~~~go
func TestFeature(t *testing.T) {
    tests := []struct {
        name string
        in   string
        want string
    }{
        {"case 1", "in1", "out1"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := Feature(tt.in); got != tt.want {
                t.Fatalf("got %q, want %q", got, tt.want)
            }
        })
    }
}
~~~

### Моки

Моки должны реализовывать интерфейс и использоваться в тесте, а не объявляться «для вида». Для web-api уже есть `database.DB`; для collector интерфейс нужно ввести (план, QUA-1).

### SQL

- `DATA-DOG/go-sqlmock` — для проверки запросов и ошибок без БД.
- Testcontainers/PostgreSQL — для интеграционных тестов схемы, upsert и партиций (план, QUA-2).

### Временные файлы

`t.TempDir()`, `t.Cleanup()`, `t.Helper()`.

## Troubleshooting

~~~bash
go clean -testcache
go mod download
~~~

- `operation not permitted` в `GOCACHE` — задайте `GOCACHE=$PWD/.gocache`.
- `race detector` требует `-race` и может замедлить тесты.
- Если тест зависит от сети/DNS — используйте `httptest`/fake resolver, а не реальный DNS.
