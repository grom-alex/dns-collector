# Development Workflow

> **Канонический регламент:** [../CONTRIBUTING.md](../CONTRIBUTING.md).
> Этот файл — короткая памятка для Claude Code; правила не дублируются.

## Перед push

1. Прогнать тесты и race:
   ~~~bash
   make test
   cd dns-collector && go test -race ./internal/... ./cmd/...
   cd ../web-api && go test -race ./internal/... ./cmd/...
   ~~~
2. Прогнать линтеры:
   ~~~bash
   cd dns-collector && make lint
   cd ../web-api && make lint
   ~~~
3. Собрать и проверить compose:
   ~~~bash
   cd ..
   make build
   docker compose config -q
   docker compose -f deploy/production/docker-compose.yml config -q
   ~~~
4. Обновить `CHANGELOG.md` (раздел `Unreleased` или версия релиза).
5. Создать PR по [.github/pull_request_template.md](../.github/pull_request_template.md); ветка — от `main`, имя по [CONTRIBUTING.md](../CONTRIBUTING.md#1-базовая-модель).

## Запрещено

- push в `main` без PR;
- деплой без бэкапа БД;
- секреты в git;
- красный CI при мерже.

## Ссылки

- [CONTRIBUTING.md](../CONTRIBUTING.md) — ветки, PR, CI, релиз, деплой, откат.
- [TESTING.md](../TESTING.md) — тесты и покрытие.
- [DEPLOYMENT.md](../DEPLOYMENT.md) — деплой.
