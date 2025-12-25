# Release Notes v2.4.0

## 🎯 Основные возможности

### 1. 🔢 Раздельная фильтрация IPv4/IPv6
Теперь можно выборочно экспортировать только IPv4 или только IPv6 адреса:

```yaml
export_lists:
  - name: "Ad Blocklist IPv4"
    endpoint: "/export/ads-ipv4"
    domain_regex: "^(ads|adservice)\\."
    include_domains: true
    include_ipv4: true
    include_ipv6: false  # Только IPv4
```

**Зачем это нужно:**
- Firewall без поддержки IPv6
- Раздельные правила для IPv4 и IPv6
- Оптимизация размера списков

### 2. 🛡️ Исключение Shared IP
Защита от случайной блокировки легитимных сервисов на CDN и облачных платформах:

```yaml
export_lists:
  - name: "Tracking Blocklist Safe"
    endpoint: "/export/tracking"
    domain_regex: "^(tracking|telemetry)\\."
    include_domains: true
    exclude_shared_ips: true  # Не блокируем CDN
```

**Как это работает:**
- DNS Collector видит: `ads.example.com` → `192.0.2.1`
- DNS Collector видит: `www.example.com` → `192.0.2.1`
- С `exclude_shared_ips: true` → `192.0.2.1` **исключается** (shared IP)

### 3. 📊 Endpoint для анализа исключенных IP
Аудит и отладка фильтрации с детальной информацией:

```yaml
export_lists:
  - name: "Ad Blocklist"
    endpoint: "/export/ads"
    excluded_ips_endpoint: "/export/ads-excluded"  # Анализ
    domain_regex: "^(ads|tracking)\\."
    exclude_shared_ips: true
```

**Формат вывода:**
```
# Excluded IPs (shared between matched and non-matched domains)
# Format: IP | Matched Domains | Non-Matched Domains
#
192.0.2.1 | ads.example.com, tracking.example.com | www.example.com, api.example.com
```

### 4. 📁 Дополнительные статические IP из файла
Интеграция threat intelligence feeds:

```yaml
export_lists:
  - name: "Malware Comprehensive"
    endpoint: "/export/malware"
    domain_regex: "\\.(malware|virus|trojan)\\."
    include_domains: false
    additional_ips_file: "/app/config/threat-intel-ips.txt"
```

**Формат файла:**
```
# Threat Intelligence Feed
# Known C&C servers
198.51.100.10
198.51.100.11

# IPv6 malware
2001:db8:bad::1
```

## 🔧 Технические улучшения

- **PostgreSQL CTEs** для эффективной фильтрации shared IP
- **Новый пакет** `internal/utils/ip_parser` с валидацией
- **Безопасность**: валидация путей, лимиты на размер файлов
- **Тесты**: новые unit-тесты для IP parser
- **Исправлен** баг с closure capture в регистрации роутов

## 📚 Документация

- Обновлена [`web-api/EXPORT_LISTS.md`](web-api/EXPORT_LISTS.md) с 5 практическими примерами
- Добавлен troubleshooting guide
- Примеры конфигурации для production в `deploy/production/config/`
- Примеры файлов: `threat-intel-ips.txt.example`, `corporate-manual-blocks.txt.example`
- Подробная инструкция в `README-additional-ips.md`

## 🔄 Обратная совместимость

✅ Все новые параметры **опциональные** с разумными defaults  
✅ Существующие конфигурации работают **без изменений**

## 📦 Deployment

```bash
# Docker images опубликованы в registry
registry.gromas.ru/apps/dns-collector/dns-collector:v2.4.0
registry.gromas.ru/apps/dns-collector/web-api:v2.4.0

# Production deployment
cd deploy/production
docker-compose pull
docker-compose up -d
```

## 🎓 Примеры использования

### Безопасная блокировка рекламы (IPv4 only)
```yaml
export_lists:
  - name: "Ad Blocklist IPv4 Safe"
    endpoint: "/export/ads-ipv4"
    excluded_ips_endpoint: "/export/ads-ipv4-excluded"
    domain_regex: "^(ads|adservice|adserver|doubleclick)\\."
    include_domains: true
    include_ipv4: true
    include_ipv6: false
    exclude_shared_ips: true
```

### Комплексный malware blocklist
```yaml
export_lists:
  - name: "Malware Extended"
    endpoint: "/export/malware"
    domain_regex: "\\.(malware|virus|trojan|botnet)\\."
    include_domains: false
    include_ipv4: true
    include_ipv6: true
    exclude_shared_ips: true
    additional_ips_file: "/app/config/threat-intel-ips.txt"
```

### Корпоративная политика с аудитом
```yaml
export_lists:
  - name: "Corporate Blocklist"
    endpoint: "/export/corporate"
    excluded_ips_endpoint: "/export/corporate-excluded"
    domain_regex: "^(facebook|twitter|instagram|gaming)\\."
    include_domains: true
    exclude_shared_ips: true
    additional_ips_file: "/app/config/corporate-manual-blocks.txt"
```

## 🐛 Bug Fixes

- Исправлен closure capture bug в route registration
- Улучшена обработка ошибок при чтении файлов

## 🙏 Благодарности

Спасибо всем, кто тестировал и предлагал улучшения!

---

**Full Changelog**: https://github.com/grom-alex/dns-collector/compare/v2.3.2...v2.4.0
