# DNS Collector Tools

Вспомогательные утилиты для работы с DNS Collector.

## Доступные инструменты

### check_db.go
Проверка локальной базы данных `domains.db`.

```bash
go run check_db.go
```

### check_docker_dbs.go
Проверка базы данных Docker контейнера (`docker_domains.db` и `docker_stats.db`).

```bash
go run check_docker_dbs.go
```

### check_stats.go
Проверка статистики из базы данных `stats.db`.

```bash
go run check_stats.go
```

## Сборка инструментов

Если нужно скомпилировать инструменты:

```bash
# Из корня проекта dns-collector
cd tools
go build -o check_db check_db.go
go build -o check_stats check_stats.go
go build -o check_docker_dbs check_docker_dbs.go
```
