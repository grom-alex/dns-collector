# DNS Collector Web API - Возможности

## Обзор

Web API предоставляет полнофункциональный веб-интерфейс и REST API для просмотра и анализа данных, собранных DNS Collector.

## Основные возможности

### 1. Статистика DNS запросов

#### Фильтрация
- **По IP клиента**: один или несколько IP адресов (через запятую)
- **По подсети**: CIDR нотация (например, `192.168.1.0/24`)
- **По диапазону дат**: с точностью до секунды
- **Комбинированные фильтры**: все фильтры можно применять одновременно

#### Сортировка
Сортировка по любому полю:
- ID записи
- Доменное имя
- IP клиента
- Тип запроса (cache/dns)
- Время запроса

Порядок: возрастание (asc) или убывание (desc)

#### Пагинация
- Настраиваемое количество записей на страницу (1-1000)
- Навигация по страницам
- Отображение общего количества записей

#### Отображение
- Таблица с цветовым кодированием типов запросов
- Форматирование дат и времени
- Выделение доменов и IP адресов
- Responsive дизайн для мобильных устройств

### 2. Просмотр доменов

#### Поиск по регулярным выражениям
Мощный поиск с поддержкой regex:
- `^google\.` - домены, начинающиеся с "google."
- `\.com$` - все .com домены
- `.*cloudflare.*` - домены, содержащие "cloudflare"
- `^mail\.|^smtp\.` - домены, начинающиеся с "mail." или "smtp."

#### Фильтрация
- По диапазону дат (время первого обнаружения)
- По статусу резолвинга
- Комбинированные фильтры

#### Сортировка
По любому полю:
- ID домена
- Доменное имя
- Время первого обнаружения
- Количество резолвингов
- Время последнего резолвинга

#### Просмотр IP адресов
Для каждого домена можно просмотреть:
- Все зарезолвленные IPv4 адреса
- Все зарезолвленные IPv6 адреса
- Время резолвинга каждого IP
- Цветовое разделение IPv4/IPv6

### 3. REST API

#### Endpoints

**GET /api/stats**
- Получение статистики DNS запросов
- Поддержка всех фильтров и сортировок
- Возвращает JSON с пагинацией

**GET /api/domains**
- Получение списка доменов
- Фильтрация по regex
- Поддержка сортировки и пагинации

**GET /api/domains/:id**
- Детальная информация о домене
- Включает все IP адреса
- Метаинформация о резолвинге

**GET /health**
- Health check endpoint
- Статус сервиса

**GET /export/{endpoint}**
- Экспорт IP адресов и доменов в plain text
- Фильтрация по regex для доменных имен
- Формат для pfSense firewall alias tables
- HTTP кеширование (5 минут)

#### Формат ответа

```json
{
  "data": [...],
  "total": 1234,
  "limit": 100,
  "offset": 0,
  "total_pages": 13
}
```

### 4. Веб-интерфейс

#### Навигация
- Двухстраничный SPA (Single Page Application)
- Вкладки: Statistics и Domains
- Быстрая навигация без перезагрузки

#### Фильтры
- Интуитивная форма фильтров
- Кнопки "Apply" и "Reset"
- Сохранение состояния при навигации

#### Таблицы
- Сортировка по клику на заголовок
- Индикатор направления сортировки (↑↓)
- Адаптивный дизайн
- Hover эффекты

#### Пагинация
- Кнопки Previous/Next
- Отображение текущей страницы
- Информация об общем количестве записей

## Технические особенности

### Backend

#### Производительность
- Read-only доступ к БД (безопасно для продакшена)
- Эффективные SQL запросы с индексами
- Пагинация на уровне БД
- Минимальное использование памяти

#### Безопасность
- Валидация всех входных данных
- Защита от SQL injection
- CORS конфигурация
- Read-only режим БД

#### Масштабируемость
- Stateless архитектура
- Легко масштабируется горизонтально
- Поддержка load balancing

### Frontend

#### Технологии
- Vue.js 3 (Composition API)
- Vue Router для навигации
- Axios для HTTP запросов
- date-fns для работы с датами
- Vite для сборки

#### UX/UI
- Responsive дизайн
- Современный интерфейс
- Быстрая загрузка
- Loading индикаторы
- Обработка ошибок

#### Производительность
- Lazy loading компонентов
- Оптимизированная сборка
- Минификация и сжатие
- Кеширование статических ресурсов

## Примеры использования

### Сценарий 1: Мониторинг активности клиента

**Задача**: Просмотреть все DNS запросы от конкретного IP

**Решение**:
1. Откройте Statistics
2. В поле "Client IPs" введите IP адрес
3. Нажмите "Apply Filters"
4. Отсортируйте по времени

**API**:
```bash
curl "http://localhost:8080/api/stats?client_ips=192.168.1.10&sort_by=timestamp&sort_order=desc"
```

### Сценарий 2: Поиск всех поддоменов

**Задача**: Найти все поддомены Google

**Решение**:
1. Откройте Domains
2. В поле "Domain Regex Pattern" введите: `.*google.*`
3. Нажмите "Apply Filters"

**API**:
```bash
curl "http://localhost:8080/api/domains?domain_regex=.*google.*"
```

### Сценарий 3: Анализ активности подсети

**Задача**: Посмотреть активность всей подсети за последний час

**Решение**:
1. Откройте Statistics
2. В поле "Subnet" введите: `192.168.1.0/24`
3. Установите диапазон дат (последний час)
4. Нажмите "Apply Filters"

**API**:
```bash
START=$(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%SZ)
END=$(date -u +%Y-%m-%dT%H:%M:%SZ)
curl "http://localhost:8080/api/stats?subnet=192.168.1.0/24&date_from=$START&date_to=$END"
```

### Сценарий 4: Поиск доменов с IPv6

**Задача**: Найти все домены, которые имеют IPv6 адреса

**Решение**:
1. Откройте Domains
2. Для каждого домена нажмите "Show IPs"
3. Домены с IPv6 будут показаны с фиолетовой меткой

**API**:
```bash
# Получить все домены, затем для каждого проверить IPs
curl "http://localhost:8080/api/domains" | jq '.data[].id' | while read id; do
  curl "http://localhost:8080/api/domains/$id" | jq 'select(.ips[] | select(.type == "ipv6"))'
done
```

### Сценарий 5: Топ популярных доменов

**Задача**: Найти наиболее часто запрашиваемые домены

**Решение**:
1. Откройте Statistics
2. Кликните на "Domain" для сортировки
3. Визуально определите часто встречающиеся домены

**Или используйте прямой SQL запрос**:
```bash
sqlite3 data/stats.db "SELECT domain, COUNT(*) as count FROM domain_stat GROUP BY domain ORDER BY count DESC LIMIT 10;"
```

### Сценарий 6: Экспорт списков для pfSense

**Задача**: Настроить автоматическое обновление firewall alias в pfSense

**Решение**:
1. Настройте export list в `config/config.yaml`:
```yaml
export_lists:
  - name: "Blocked Domains"
    endpoint: "/export/blocked"
    domain_regex: "^(ads|tracking|malware)\\."
    include_domains: true
```

2. В pfSense создайте URL Table Alias:
   - Type: URL Table (IPs or Hostnames)
   - URL: `http://your-server:8080/export/blocked`
   - Update Frequency: 300 (5 минут)

3. Используйте alias в firewall rules для блокировки

**API**:
```bash
# Получение списка доменов и IP
curl "http://localhost:8080/export/blocked"

# Вывод:
# ads.example.com
# tracking.analytics.com
# 192.0.2.1
# 192.0.2.2
# 2001:db8::1
```

## Расширенные возможности

### Интеграция с другими системами

#### Через REST API
```python
import requests

# Получение статистики
response = requests.get('http://localhost:8080/api/stats', params={
    'client_ips': '192.168.1.10',
    'limit': 100
})
stats = response.json()

# Обработка данных
for record in stats['data']:
    print(f"{record['timestamp']}: {record['domain']} from {record['client_ip']}")
```

#### Через SQL (прямой доступ к БД)
```python
import sqlite3

conn = sqlite3.connect('data/stats.db')
cursor = conn.cursor()

cursor.execute("""
    SELECT domain, COUNT(*) as count
    FROM domain_stat
    WHERE timestamp >= datetime('now', '-1 hour')
    GROUP BY domain
    ORDER BY count DESC
    LIMIT 10
""")

for domain, count in cursor.fetchall():
    print(f"{domain}: {count} запросов")
```

### Экспорт данных

#### JSON
```bash
# Экспорт всех доменов
curl "http://localhost:8080/api/domains?limit=10000" > domains.json
```

#### CSV (через jq)
```bash
# Экспорт статистики в CSV
curl "http://localhost:8080/api/stats?limit=10000" | \
  jq -r '.data[] | [.id, .domain, .client_ip, .rtype, .timestamp] | @csv' > stats.csv
```

### Автоматизация

#### Периодический мониторинг
```bash
#!/bin/bash
# monitor_subnet.sh - мониторинг активности подсети

while true; do
  COUNT=$(curl -s "http://localhost:8080/api/stats?subnet=192.168.1.0/24&limit=1" | jq '.total')
  echo "$(date): Запросов из подсети: $COUNT"
  sleep 300
done
```

#### Алерты
```bash
#!/bin/bash
# alert_new_domains.sh - уведомление о новых доменах

LAST_COUNT=0

while true; do
  COUNT=$(curl -s "http://localhost:8080/api/domains?limit=1" | jq '.total')

  if [ $COUNT -gt $LAST_COUNT ]; then
    NEW=$((COUNT - LAST_COUNT))
    echo "ALERT: Обнаружено $NEW новых доменов!"
    # Отправить уведомление
  fi

  LAST_COUNT=$COUNT
  sleep 60
done
```

### pfSense Firewall Integration

**Реализовано в v2.3.0, расширено в v2.4.0**: Полная интеграция с pfSense firewall через export lists

#### Возможности
- Экспорт доменов и IP в plain text формате
- Фильтрация по регулярным выражениям (PostgreSQL синтаксис)
- Настраиваемое включение/выключение доменов
- **NEW in v2.4.0**: Раздельная фильтрация IPv4 и IPv6 адресов
- **NEW in v2.4.0**: Исключение shared IP адресов (CDN, облачные сервисы)
- **NEW in v2.4.0**: Endpoint для анализа исключенных IP
- **NEW in v2.4.0**: Добавление статических IP из внешних файлов
- Поддержка множественных списков
- Автоматическая сортировка: домены → IPv4 → IPv6
- HTTP кеширование для снижения нагрузки
- Защита от ReDoS атак

#### Безопасность
- Валидация regex (длина ≤ 200 символов)
- Блокировка опасных паттернов: `(.*)*`, `(.+)+`, `(.*)+`, `(.+)*`
- Проверка дубликатов имен и endpoints
- Валидация конфигурации при старте

#### Примеры использования

**Блокировка рекламных доменов:**
```yaml
export_lists:
  - name: "Ad Blocklist"
    endpoint: "/export/ads"
    domain_regex: "^(ads|adservice|analytics|tracking)\\."
    include_domains: true
```

**Экспорт только IP адресов CDN:**
```yaml
export_lists:
  - name: "CDN IPs"
    endpoint: "/export/cdn-ips"
    domain_regex: "\\.(cloudflare|akamai|fastly)\\.com$"
    include_domains: false
```

**Фильтрация по TLD:**
```yaml
export_lists:
  - name: "RU Domains"
    endpoint: "/export/ru"
    domain_regex: "\\.ru$"
    include_domains: true
```

**NEW in v2.4.0 - IPv4-only с исключением shared IP:**
```yaml
export_lists:
  - name: "Ad Blocklist IPv4 Safe"
    endpoint: "/export/ads-ipv4"
    excluded_ips_endpoint: "/export/ads-ipv4-excluded"
    domain_regex: "^(ads|adservice|tracking)\\."
    include_domains: true
    include_ipv4: true
    include_ipv6: false
    exclude_shared_ips: true
```

**NEW in v2.4.0 - Расширенный blocklist с threat intelligence:**
```yaml
export_lists:
  - name: "Malware Extended"
    endpoint: "/export/malware"
    domain_regex: "\\.(malware|virus|trojan)\\."
    include_domains: false
    include_ipv4: true
    include_ipv6: true
    exclude_shared_ips: true
    additional_ips_file: "/app/config/threat-intel-ips.txt"
```

Подробнее: [`web-api/EXPORT_LISTS.md`](EXPORT_LISTS.md)

### 5. Excel экспорт (v2.3.2+)

#### Экспорт статистики в Excel
- **Endpoint**: `GET /api/stats/export`
- **Формат**: Excel (.xlsx) с одним листом "DNS Statistics"
- **Применение фильтров**: все фильтры из веб-интерфейса применяются к экспорту
- **Лимит**: 100,000 записей (HTTP 413 при превышении)

**Колонки:**
- ID - уникальный идентификатор записи
- Domain - доменное имя
- Client IP - IP адрес клиента
- Record Type - тип записи DNS
- Timestamp - время запроса (yyyy-mm-dd hh:mm:ss)

**Форматирование:**
- Жирные заголовки с синим фоном (#4A90E2)
- Закрепление первой строки (freeze panes)
- Автофильтры на всех колонках
- Оптимизированная ширина колонок
- Профессиональный формат дат

#### Экспорт доменов в Excel
- **Endpoint**: `GET /api/domains/export`
- **Формат**: Excel (.xlsx) с двумя листами
- **Оптимизация**: bulk fetch IP адресов (один SQL запрос для всех доменов)
- **Лимит**: 100,000 записей

**Лист 1 "Domains":**
- ID, Domain, First Seen, Resolution Count, Max Resolutions, Last Resolved

**Лист 2 "IP Addresses":**
- Domain, IP Address, Type (IPv4/IPv6), Resolved At

**Особенности:**
- Полное форматирование на обоих листах
- Связь доменов с IP адресами через доменное имя
- Удобная фильтрация и анализ в Excel

#### Интеграция в веб-интерфейс
- Кнопки "Export to Excel" в блоке фильтров
- Индикаторы загрузки при экспорте
- Автоматическая загрузка файла с правильным именем
- Обработка ошибок (слишком большой датасет, ошибки сервера)

#### Примеры использования

**Экспорт всей статистики:**
```bash
curl -o dns-stats.xlsx "http://localhost:8080/api/stats/export"
```

**Экспорт с фильтрами:**
```bash
# Только запросы от конкретного IP
curl -o client-stats.xlsx "http://localhost:8080/api/stats/export?client_ips=192.168.1.10"

# Запросы из подсети за определенный период
curl -o subnet-stats.xlsx "http://localhost:8080/api/stats/export?subnet=192.168.1.0/24&date_from=2024-12-01T00:00:00Z"
```

**Экспорт доменов:**
```bash
# Все домены
curl -o all-domains.xlsx "http://localhost:8080/api/domains/export"

# Домены, содержащие "google"
curl -o google-domains.xlsx "http://localhost:8080/api/domains/export?domain_regex=.*google.*"
```

## Будущие улучшения

### Планируемые возможности
- Dashboard с графиками и статистикой
- ~~Экспорт в Excel (XLSX)~~ ✅ **Реализовано в v2.3.2**
- Экспорт в CSV формат
- WebSocket для real-time обновлений
- Сохранение пользовательских фильтров
- Темная тема интерфейса
- Графики активности по времени
- Geolocation IP адресов
- Алерты и уведомления
- Rate limiting для export endpoints
- Query timeout для сложных regex

### Возможные интеграции
- Prometheus/Grafana для метрик
- Elasticsearch для full-text поиска
- Redis для кеширования
- Kafka для потоковой обработки
- Integration tests с testcontainers-go
