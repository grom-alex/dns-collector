# Как вести доработки DNS Collector

Это обязательный регламент для изменений в репозитории. Он заменяет устаревшие инструкции в `.claude/DEVELOPMENT_WORKFLOW.md`.

## 1. Базовая модель

- `main` — всегда рабочая и потенциально разворачиваемая ветка.
- Прямые push в `main` запрещены; изменения — только через PR.
- Одна ветка = одна логическая доработка. Не смешивайте рефакторинг, фичу и обновление документации в одном PR.
- Короткоживущие ветки: дни, не недели. Регулярно синхронизируйтесь с `main`.

Имена веток:

| Префикс | Назначение | Пример |
|---|---|---|
| `fix/` | баг, регресс, оптимизация | `fix/db-write-path` |
| `feat/` | новая функциональность | `feat/domain-cache` |
| `docs/` | только документация | `docs/workflow` |
| `chore/` | CI, зависимости, инфраструктура | `chore/ci-go124` |
| `hotfix/` | срочное исправление от тега | `hotfix/2.8.1` |

## 2. Цикл доработки

1. **Зафиксируйте задачу.** Issue или хотя бы абзац в PR: что меняем, зачем, критерий готовности.
2. **Обновите `main` и создайте ветку:**
   ~~~bash
   git fetch origin
   git switch main
   git pull --ff-only origin main
   git switch -c fix/db-write-path
   ~~~
3. **Внесите изменения.** Код, тесты, миграции, документация — в одной ветке.
4. **Добавьте запись в `CHANGELOG.md`** в раздел `Unreleased` (или в версию релиза).
5. **Прогоните локальные проверки** (раздел 3).
6. **Создайте PR в `main`** по шаблону [.github/pull_request_template.md](.github/pull_request_template.md).
7. **Дождитесь зелёного CI и ревью.** Красный CI — не мержим.
8. **Смержите PR** (merge commit, как в истории проекта), удалите ветку.
9. **Релиз и деплой** — по разделам 6–7, только после отдельного решения.

## 3. Локальные проверки перед push

Минимум:

~~~bash
# оба сервиса
make test

# race detector (как в CI)
cd dns-collector && go test -race ./internal/... ./cmd/...
cd ../web-api && go test -race ./internal/... ./cmd/...

# линтеры
cd dns-collector && make lint
cd ../web-api && make lint

# сборка и compose
cd ..
make build
docker compose config -q
docker compose -f deploy/production/docker-compose.yml config -q
~~~

Если менялись миграции или SQL — дополнительно проверьте на копии БД, а не на production.

Если менялся frontend:

~~~bash
cd web-api/frontend
npm ci
npm run build
~~~

Если менялись Dockerfile/зависимости — соберите образы:

~~~bash
docker build -t dns-collector:test ./dns-collector
docker build -t dns-collector-webapi:test ./web-api
~~~

## 4. Pull request

Требования:

- Заголовок в стиле `fix: ...` / `feat: ...` / `docs: ...`.
- Описание: что, зачем, как проверено, риски, влияние на БД/деплой.
- Ссылка на issue/задачу, если есть.
- Для UI — скриншот до/после.
- Для БД — план миграции и откат.
- CI должен быть зелёным.
- Минимум одно ревью. Для solo-разработки — self-review + зелёный CI; лучше попросить внешний review для рискованных изменений.

Не пушьте force в общую ветку после начала ревью без повторного запроса.

## 5. CI

Канонический workflow — `.github/workflows/test.yml`:

- `go test -v -race` для обоих сервисов;
- `golangci-lint`;
- `go build`;
- Codecov.

Требования к настройкам GitHub:

- `main` protected;
- обязательные checks из `Tests`;
- запрет прямых push;
- включённый `Require a pull request before merging`.

Текущие пробелы, которые надо закрыть отдельным PR (QUA-3 в плане):

- `.github/workflows/build.yml` — legacy: Go 1.21, `libsqlite3`, `continue-on-error: true`; либо удалить, либо привести к `test.yml`.
- PR template добавлен; branch protection нужно включить в настройках GitHub (в репозитории это не задаётся).
- Frontend не имеет тестов/линтера.
- `release.yml` теперь публикует Docker-образы; для этого в GitHub должны быть заданы secrets `REGISTRY_USERNAME`/`REGISTRY_PASSWORD`.

До исправления CI считаем обязательным `test.yml`, а `build.yml` — вспомогательным.

## 6. Версии, релиз, публикация

Семантическое версионирование `MAJOR.MINOR.PATCH`:

- MAJOR — несовместимые изменения;
- MINOR — новая функциональность;
- PATCH — исправления.

Порядок релиза:

1. Подготовьте **release PR** в `main`: обновите `CHANGELOG.md` (версия, дата, изменения, миграции), версии образов в `deploy/production/docker-compose.yml` и, при необходимости, в `docker-compose.yml`.
2. После merge release PR создайте аннотированный тег на merge-коммите `main` и запушьте только тег:
   ~~~bash
   git switch main
   git pull --ff-only origin main
   git tag -a vX.Y.Z -m "Release vX.Y.Z"
   git push origin vX.Y.Z
   ~~~
3. Workflow [.github/workflows/release.yml](.github/workflows/release.yml) на тег `v*`:
   - собирает и публикует Docker-образы `X.Y.Z` и `latest` (для стабильных тегов) в `registry.gromas.ru`;
   - собирает бинарники;
   - создаёт GitHub Release.
4. Дождитесь зелёного `Release`. Проверьте образы:
   ~~~bash
   docker pull registry.gromas.ru/apps/dns-collector/dns-collector:X.Y.Z
   docker pull registry.gromas.ru/apps/dns-collector/web-api:X.Y.Z
   ~~~
5. Убедитесь, что GitHub Release создан, а теги образов соответствуют версии.

Если CI недоступен, допустима ручная публикация с машины, где выполнен `docker login registry.gromas.ru`:

~~~bash
docker build --no-cache \
  -t registry.gromas.ru/apps/dns-collector/dns-collector:X.Y.Z \
  -t registry.gromas.ru/apps/dns-collector/dns-collector:latest ./dns-collector
docker build --no-cache \
  -t registry.gromas.ru/apps/dns-collector/web-api:X.Y.Z \
  -t registry.gromas.ru/apps/dns-collector/web-api:latest ./web-api
docker push registry.gromas.ru/apps/dns-collector/dns-collector:X.Y.Z
docker push registry.gromas.ru/apps/dns-collector/dns-collector:latest
docker push registry.gromas.ru/apps/dns-collector/web-api:X.Y.Z
docker push registry.gromas.ru/apps/dns-collector/web-api:latest
~~~

Секреты CI: `REGISTRY_USERNAME` и `REGISTRY_PASSWORD` (personal access token registry с правом push). Не публикуйте `latest` вручную для предварительных версий.

## 7. Деплой

Production: `192.168.0.15`, каталог `/opt/dns-collector`, compose-проект `dns-collector`.

Перед деплоем:

~~~bash
cd /opt/dns-collector
mkdir -p backups

# Бэкап БД
docker compose exec -T postgres \
  pg_dump -U dns_collector -d dns_collector -Fc \
  > "backups/dns_$(date +%F_%H%M).dump"

# Свободное место и здоровье
df -h / /mnt/pgdata
docker compose ps
~~~

Деплой:

~~~bash
cd /opt/dns-collector
docker compose pull
docker compose up -d
docker compose ps
docker compose logs --tail=100 dns-collector
~~~

Миграции применяются при старте сервисов. Если миграция упала — сервис не стартует; не игнорировать.

Smoke-тест:

~~~bash
curl -fsS http://localhost:8080/health
echo '{"client_ip":"127.0.0.1","domain":"example.com","qtype":"A","rtype":"dns"}' \
  | nc -u -w1 localhost 5353
docker compose exec -T postgres psql -U dns_collector -d dns_collector \
  -c "SELECT count(*) FROM domain_stat;"
~~~

Откат:

- вернуть предыдущий тег в `deploy/production/docker-compose.yml`;
- `docker compose up -d`;
- если миграция несовместима — восстановить дамп из бэкапа;
- проверить health и логи.

## 8. Изменения БД

- Только через миграции `dns-collector/internal/database/migrations` и `web-api/internal/database/migrations`; не менять схему руками.
- Обе копии миграций должны совпадать.
- Не удалять и не переименовывать колонки в том же релизе, где добавлена новая: сначала expand (добавить), потом backfill, потом contract (удалить) — минимум через один релиз.
- Перед миграцией — бэкап и проверка на копии БД.
- Не использовать `ALTER DATABASE` с жёстким именем БД в миграциях.
- Для тяжёлых изменений — окно обслуживания и явный rollback-план.

## 9. Hotfix

1. Ветка от тега последнего релиза: `git switch -c hotfix/2.9.1 v2.9.0`.
2. Минимальное исправление + тест.
3. PR в `main`, зелёный CI.
4. Тег `v2.8.1`, публикация образов, деплой по разделу 7.

## 10. Запрещено

- Прямой push в `main` и force-push в общие ветки.
- Деплой без бэкапа БД.
- Ручные `UPDATE/DELETE` в production без бэкапа и согласования.
- Секреты в git (пароли, токены, ключи).
- Смешивать в одном PR несвязанные изменения.
- Оставлять красный CI или падающие тесты «на потом».

## 11. Текущее состояние

- Регламент зафиксирован в этом файле.
- `.claude/DEVELOPMENT_WORKFLOW.md` оставлен как указатель на этот документ.
- PR template: [.github/pull_request_template.md](.github/pull_request_template.md).
- CI-пробелы перечислены в разделе 5; Docker-публикация в `release.yml` добавлена, остаются `build.yml`, branch protection и frontend-проверки (QUA-3).
- Деплой-документация: [DEPLOYMENT.md](DEPLOYMENT.md), [deploy/production/README.md](deploy/production/README.md); версии синхронизированы с 2.9.0.
