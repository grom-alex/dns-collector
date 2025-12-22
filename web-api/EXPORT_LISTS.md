# Export Lists для pfSense

## Описание

Функциональность экспорта списков IP-адресов и доменов в формате plain text для использования в таблицах алиасов pfSense.

## Возможности

- Экспорт доменов и IP-адресов по HTTP в формате plain text
- Фильтрация по регулярному выражению для доменных имен
- Настраиваемое включение/выключение доменов в выгрузке
- Поддержка множественных списков с разными критериями
- Автоматическая сортировка: домены, IPv4, IPv6

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

- `name` (обязательный) - Название списка для идентификации
- `endpoint` (обязательный) - HTTP endpoint для получения списка (должен начинаться с `/`)
- `domain_regex` (обязательный) - Регулярное выражение для фильтрации доменов (PostgreSQL regex синтаксис)
- `include_domains` (обязательный) - Флаг включения доменов в выгрузку (true/false)

### Ограничения

- Длина регулярного выражения не более 200 символов
- Запрещены потенциально опасные паттерны regex (для защиты от ReDoS атак)
- Имена списков и endpoints должны быть уникальными

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
