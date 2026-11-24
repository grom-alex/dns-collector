# Export Lists для pfSense

## Описание

Функциональность экспорта списков IP-адресов и доменов в формате plain text для использования в таблицах алиасов pfSense.

## Возможности

- Экспорт доменов и IP-адресов по HTTP в формате plain text
- Фильтрация по регулярному выражению для доменных имен
- Настраиваемое включение/выключение доменов в выгрузке
- **Раздельная фильтрация IPv4 и IPv6 адресов**
- **Исключение shared IP адресов (используемых несколькими доменами)**
- **Отдельный endpoint для анализа исключенных IP**
- **Добавление статических IP из внешнего файла**
- Поддержка множественных списков с разными критериями
- Автоматическая сортировка: домены, IPv4, IPv6
- HTTP кеширование (5 минут)

## Конфигурация

Списки настраиваются в секции `export_lists` конфигурационного файла:

```yaml
export_lists:
  - name: "Example Domain List"
    endpoint: "/export/example"
    domain_regex: "^example\\.com$"
    include_domains: true
  - name: "Google Services"
    endpoint: "/export/google"
    domain_regex: ".*\\.google\\.com$"
    include_domains: true
  - name: "All Domains IPs Only"
    endpoint: "/export/all-ips"
    domain_regex: ".*"
    include_domains: false
```

### Параметры конфигурации

#### Обязательные параметры

- `name` - Название списка для идентификации
- `endpoint` - HTTP endpoint для получения списка (должен начинаться с `/`)
- `domain_regex` - Регулярное выражение для фильтрации доменов (PostgreSQL regex синтаксис)
- `include_domains` - Флаг включения доменов в выгрузку (true/false)

#### Опциональные параметры (новые в v2.4.0)

- `include_ipv4` - Включить IPv4 адреса в экспорт (default: `true`)
- `include_ipv6` - Включить IPv6 адреса в экспорт (default: `true`)
- `exclude_shared_ips` - Исключить IP адреса, которые используются как доменами из списка, так и доменами вне списка (default: `false`)
- `excluded_ips_endpoint` - Endpoint для получения списка исключенных IP с деталями (опционально)
- `additional_ips_file` - Путь к файлу со статическими IP адресами для добавления в экспорт (опционально)

### Ограничения

- Длина регулярного выражения не более 200 символов
- Запрещены потенциально опасные паттерны regex (для защиты от ReDoS атак)
- Имена списков и endpoints должны быть уникальными
- Путь к `additional_ips_file` должен быть абсолютным и находиться в `/app/config/`
- Максимум 100,000 строк в файле дополнительных IP

## Новые возможности (v2.4.0)

### 1. Фильтрация IPv4/IPv6

Позволяет выбрать какие типы IP адресов включать в экспорт. Полезно когда:
- Firewall не поддерживает IPv6
- Нужны раздельные списки для IPv4 и IPv6 правил
- Оптимизация размера списка

**Пример**: Только IPv4 адреса
```yaml
export_lists:
  - name: "IPv4 Only List"
    endpoint: "/export/ipv4-only"
    domain_regex: ".*"
    include_domains: true
    include_ipv4: true
    include_ipv6: false
```

### 2. Исключение Shared IP (exclude_shared_ips)

Исключает из экспорта IP адреса, которые используются **одновременно** доменами из списка И доменами вне списка.

**Назначение**: Предотвратить блокировку легитимных сервисов, которые используют те же IP что и нежелательные домены (например, CDN, облачные провайдеры).

**Пример**: Блокировка рекламы без вреда основным сайтам
```yaml
export_lists:
  - name: "Ad Blocklist Safe"
    endpoint: "/export/ads"
    domain_regex: "^(ads|adservice|tracking)\\."
    include_domains: true
    exclude_shared_ips: true  # Блокируем только уникальные IP рекламных доменов
```

**Как работает**:
- DNS Collector видит: `ads.example.com` → `192.0.2.1`
- DNS Collector видит: `www.example.com` → `192.0.2.1`
- С `exclude_shared_ips: false` → `192.0.2.1` включается в экспорт
- С `exclude_shared_ips: true` → `192.0.2.1` **исключается** (shared IP)

### 3. Endpoint для анализа исключенных IP

Предоставляет детальную информацию о том, какие IP были исключены и почему.

**Назначение**:
- Аудит и отладка фильтрации
- Принятие решений о безопасности блокировки
- Анализ инфраструктуры доменов

**Формат вывода**:
```
# Excluded IPs (shared between matched and non-matched domains)
# Format: IP | Matched Domains | Non-Matched Domains
#
192.0.2.1 | ads.example.com, tracking.example.com | www.example.com, api.example.com
```

**Пример конфигурации**:
```yaml
export_lists:
  - name: "Ad Blocklist with Analysis"
    endpoint: "/export/ads"
    excluded_ips_endpoint: "/export/ads-excluded"  # Endpoint для анализа
    domain_regex: "^(ads|tracking)\\."
    include_domains: true
    exclude_shared_ips: true
```

**Использование**:
```bash
# Получить основной blocklist
curl http://localhost:8080/export/ads > ads-blocklist.txt

# Проанализировать какие IP были исключены
curl http://localhost:8080/export/ads-excluded
```

### 4. Дополнительные статические IP (additional_ips_file)

Добавляет IP адреса из внешнего файла в экспорт.

**Назначение**:
- Добавление известных вредоносных IP из threat intelligence feeds
- Ручные blacklist/whitelist
- Статические IP, не связанные с доменами в БД

**Формат файла** (`/app/config/additional-ips.txt`):
```
# Комментарии начинаются с #
192.0.2.100
192.0.2.101

# Known malware C&C servers
203.0.113.50
203.0.113.51

# IPv6 addresses
2001:db8::100
```

**Пример конфигурации**:
```yaml
export_lists:
  - name: "Malware Blocklist Extended"
    endpoint: "/export/malware"
    domain_regex: "\\.(malware|virus|trojan)\\."
    include_domains: false
    additional_ips_file: "/app/config/known-malware-ips.txt"
```

**Особенности**:
- Файл читается при каждом запросе экспорта
- Пустые строки и комментарии (`#`) игнорируются
- Невалидные IP пропускаются с предупреждением в логах
- Фильтрация по `include_ipv4` и `include_ipv6` применяется
- Автоматическая дедупликация с IP из БД

## Использование

### Пример запроса

```bash
curl http://localhost:8080/export/example
```

### Формат ответа

```
example.com
203.0.113.5
198.51.100.1
2001:db8::1
```

### Порядок данных в ответе

1. Домены (если `include_domains: true`)
2. IPv4 адреса
3. IPv6 адреса

Каждая запись на отдельной строке, без пустых строк между секциями.

## Интеграция с pfSense

1. В pfSense перейдите в **Firewall > Aliases**
2. Создайте новый алиас типа "URL Table (IPs)" или "URL Table"
3. В поле URL укажите адрес вашего эндпоинта, например:
   ```
   http://dns-collector-api:8080/export/example
   ```
4. Настройте интервал обновления (Update Frequency)
5. Сохраните и примените изменения

## Примеры регулярных выражений

### Точное совпадение домена
```yaml
domain_regex: "^example\\.com$"
```

### Все поддомены
```yaml
domain_regex: ".*\\.example\\.com$"
```

### Домены с определенным окончанием
```yaml
domain_regex: ".*\\.ru$"
```

### Несколько доменов (OR)
```yaml
domain_regex: "^(example\\.com|test\\.org|demo\\.net)$"
```

### Все домены
```yaml
domain_regex: ".*"
```

## Валидация и безопасность

- При запуске сервиса выполняется валидация всех настроенных списков
- Проверяется корректность регулярных выражений
- Блокируются потенциально опасные regex паттерны
- Проверяется уникальность имен и endpoints
- Content-Type ответа всегда `text/plain; charset=utf-8`

## Логирование

При старте сервиса для каждого настроенного списка выводится сообщение:

```
Registered export list 'Example Domain List' at /export/example
```

Ошибки при обработке запросов логируются с указанием деталей.

## Мониторинг

Рекомендуется настроить мониторинг доступности эндпоинтов:

```bash
# Проверка доступности
curl -I http://localhost:8080/export/example

# Проверка содержимого
curl http://localhost:8080/export/example | head -n 10
```

## Практические примеры

### Пример 1: Блокировка рекламы (IPv4 only, безопасная)

**Задача**: Блокировать рекламные домены, но не блокировать легитимные сервисы на shared IP.

```yaml
export_lists:
  - name: "Ad Blocklist IPv4"
    endpoint: "/export/ads-ipv4"
    excluded_ips_endpoint: "/export/ads-ipv4-excluded"
    domain_regex: "^(ads|adservice|adserver|doubleclick|googlesyndication)\\."
    include_domains: true
    include_ipv4: true
    include_ipv6: false  # Firewall не поддерживает IPv6
    exclude_shared_ips: true  # Не блокируем CDN и облачные сервисы
```

**Использование**:
```bash
# Загрузить в pfSense Alias
curl http://dns-api:8080/export/ads-ipv4 > /var/db/aliastables/ads_blocklist.txt

# Проверить что было исключено
curl http://dns-api:8080/export/ads-ipv4-excluded
```

### Пример 2: Комплексный malware blocklist

**Задача**: Блокировать известные вредоносные домены + статические IP из threat intel.

```yaml
export_lists:
  - name: "Malware Comprehensive"
    endpoint: "/export/malware-all"
    domain_regex: "\\.(malware|virus|trojan|botnet|ransomware|phish)\\."
    include_domains: false  # Только IP, домены не нужны
    include_ipv4: true
    include_ipv6: true
    exclude_shared_ips: true
    additional_ips_file: "/app/config/threat-intel-ips.txt"
```

**Файл `/app/config/threat-intel-ips.txt`**:
```
# Threat Intelligence Feed - Updated 2025-12-24
# Source: Example Threat Intel Provider

# Known C&C servers
198.51.100.10
198.51.100.11
203.0.113.50

# Botnet infrastructure
192.0.2.100
192.0.2.101

# IPv6 malware
2001:db8:bad::1
2001:db8:bad::2
```

### Пример 3: Раздельные списки для IPv4 и IPv6

**Задача**: Создать отдельные pfSense aliases для IPv4 и IPv6.

```yaml
export_lists:
  # IPv4 список
  - name: "Tracking IPv4"
    endpoint: "/export/tracking-v4"
    domain_regex: "^(tracking|analytics|telemetry)\\."
    include_domains: false
    include_ipv4: true
    include_ipv6: false

  # IPv6 список
  - name: "Tracking IPv6"
    endpoint: "/export/tracking-v6"
    domain_regex: "^(tracking|analytics|telemetry)\\."
    include_domains: false
    include_ipv4: false
    include_ipv6: true
```

### Пример 4: Полный контроль с анализом

**Задача**: Максимально гибкая конфигурация с возможностью аудита.

```yaml
export_lists:
  - name: "Corporate Blocklist"
    endpoint: "/export/corporate-block"
    excluded_ips_endpoint: "/export/corporate-block-excluded"
    domain_regex: "^(social-media|gaming|streaming|adult)\\."
    include_domains: true
    include_ipv4: true
    include_ipv6: true
    exclude_shared_ips: true
    additional_ips_file: "/app/config/corporate-manual-block.txt"
```

**Workflow**:
1. Регулярно проверять excluded endpoint для анализа
2. Принимать решения о том, стоит ли блокировать shared IP
3. Добавлять подтвержденные IP в `corporate-manual-block.txt`
4. pfSense автоматически обновляет alias

### Пример 5: Legacy конфигурация (обратная совместимость)

**Задача**: Старая конфигурация должна работать без изменений.

```yaml
export_lists:
  - name: "Legacy List"
    endpoint: "/export/legacy"
    domain_regex: "^example\\."
    include_domains: true
    # Все новые параметры опциональные - используются defaults:
    # include_ipv4: true (default)
    # include_ipv6: true (default)
    # exclude_shared_ips: false (default)
```

## Устранение неполадок

### Проблема: Пустой список экспорта

**Причины**:
1. Regex не совпадает ни с одним доменом в БД
2. Все IP адреса исключены (если `exclude_shared_ips: true`)
3. `include_ipv4: false` и `include_ipv6: false` одновременно

**Решение**:
```bash
# Проверить какие домены в БД
curl http://dns-api:8080/api/domains?limit=100

# Проверить excluded IP
curl http://dns-api:8080/export/your-list-excluded

# Проверить логи
docker compose logs web-api | grep "Export list returned empty"
```

### Проблема: Файл дополнительных IP не загружается

**Причины**:
1. Путь не абсолютный
2. Файл не в `/app/config/`
3. Файл не существует
4. Неправильные права доступа

**Решение**:
```bash
# Проверить файл в контейнере
docker exec dns-collector-webapi ls -la /app/config/

# Проверить содержимое
docker exec dns-collector-webapi cat /app/config/your-file.txt

# Проверить логи
docker compose logs web-api | grep "additional"
```

### Проблема: Excluded endpoint возвращает пустой список

**Причины**:
1. `exclude_shared_ips: false` (исключения не вычисляются)
2. Нет IP, используемых одновременно matched и non-matched доменами

**Решение**: Это нормально, если shared IP действительно нет. Проверьте:
```bash
# Посмотреть основной экспорт
curl http://dns-api:8080/export/your-list

# Если в нем есть IP, но excluded пустой - значит все IP уникальные
```

## Changelog

### v2.4.0 (2025-12-24)
- ✨ Добавлена фильтрация IPv4/IPv6 (`include_ipv4`, `include_ipv6`)
- ✨ Добавлено исключение shared IP (`exclude_shared_ips`)
- ✨ Добавлен endpoint для анализа исключенных IP (`excluded_ips_endpoint`)
- ✨ Добавлена поддержка дополнительных статических IP (`additional_ips_file`)
- 🔒 Улучшена валидация конфигурации
- 📝 Обновлена документация с практическими примерами
