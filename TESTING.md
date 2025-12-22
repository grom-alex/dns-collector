# Testing Guide

Этот документ описывает тестирование проекта DNS Collector.

## Структура тестов

### dns-collector
- `internal/config/config_test.go` - Тесты конфигурации (9 тестов)
  - Загрузка и валидация YAML конфигурации
  - Переопределение через переменные окружения
  - Проверка значений по умолчанию
  - Обработка ошибок

### web-api
- `internal/handlers/handlers_test.go` - Тесты HTTP handlers (7 тестов)
  - Health check endpoint
  - Statistics API с фильтрацией и пагинацией
  - Domain API с валидацией
  - Обработка ошибок базы данных
- `internal/database/interface.go` - Интерфейс для моков в тестах

## Запуск тестов

### Используя Makefile (рекомендуется)

```bash
# Запустить все тесты
make test

# Запустить тесты только dns-collector
make test-dns-collector

# Запустить тесты только web-api
make test-web-api

# Запустить тесты с детальным выводом
make test-verbose

# Запустить тесты с coverage
make test-coverage
```

### Напрямую через go test

```bash
# dns-collector тесты
cd dns-collector
go test ./internal/... ./cmd/... -v

# web-api тесты
cd web-api
go test ./internal/... ./cmd/... -v

# С coverage
go test ./internal/... ./cmd/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Coverage отчеты

После выполнения `make test-coverage` HTML отчеты доступны:
- `dns-collector/coverage.html`
- `web-api/coverage.html`

## CI/CD

GitHub Actions автоматически запускает тесты при:
- Push в ветки `main`, `develop`, `feature/*`
- Pull requests в `main` и `develop`

Workflow выполняет:
1. **Тесты** с race detector и coverage
2. **Линтинг** через golangci-lint
3. **Сборку** обоих сервисов

Результаты coverage автоматически отправляются в Codecov.

## Написание новых тестов

### Структура теста

```go
func TestFeatureName(t *testing.T) {
    // Setup

    // Execute

    // Assert
    if got != want {
        t.Errorf("got %v, want %v", got, want)
    }
}
```

### Table-driven тесты

```go
func TestMultipleCases(t *testing.T) {
    tests := []struct {
        name string
        input string
        want string
    }{
        {"case 1", "input1", "output1"},
        {"case 2", "input2", "output2"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := processInput(tt.input)
            if got != tt.want {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Mock database

```go
type MockDatabase struct {
    GetDataFunc func(id int) (Data, error)
}

func (m *MockDatabase) GetData(id int) (Data, error) {
    if m.GetDataFunc != nil {
        return m.GetDataFunc(id)
    }
    return Data{}, nil
}
```

## Лучшие практики

1. **Используйте t.Helper()** для вспомогательных функций
2. **Не забывайте t.Parallel()** для параллельных тестов
3. **Используйте table-driven подход** для множественных сценариев
4. **Моки** должны реализовывать интерфейсы
5. **Cleanup** через `t.Cleanup()` или `defer`
6. **Временные файлы** через `t.TempDir()`

## Статистика тестов

```
dns-collector:
  - config: 9 тестов, 100% coverage

web-api:
  - handlers: 7 тестов, полное покрытие endpoints

Общее время выполнения: ~10-20ms
```

## Troubleshooting

### Тесты не запускаются

```bash
# Проверьте зависимости
go mod download
go mod tidy

# Очистите кэш
go clean -testcache
```

### Race detector warnings

```bash
# Запустите с race detector
go test -race ./...
```

### Coverage не генерируется

```bash
# Убедитесь что указаны правильные пакеты
go test ./internal/... ./cmd/... -coverprofile=coverage.out
```
