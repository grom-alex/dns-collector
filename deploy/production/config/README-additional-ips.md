# Additional IP Files Configuration

Этот каталог содержит примеры файлов с дополнительными IP адресами для функции `additional_ips_file`.

## Использование

1. Скопируйте `.example` файлы в рабочие версии:
   ```bash
   cp threat-intel-ips.txt.example threat-intel-ips.txt
   cp corporate-manual-blocks.txt.example corporate-manual-blocks.txt
   ```

2. Отредактируйте файлы, добавив реальные IP адреса

3. Настройте `web-api.yaml` для использования этих файлов:
   ```yaml
   export_lists:
     - name: "Threat Blocklist"
       endpoint: "/export/threats"
       domain_regex: ".*"
       include_domains: false
       additional_ips_file: "/app/config/threat-intel-ips.txt"
   ```

## Формат файла

- Один IP адрес на строку
- Комментарии начинаются с `#`
- Пустые строки игнорируются
- Поддерживаются IPv4 и IPv6
- **CIDR нотация НЕ поддерживается** - используйте отдельные IP

## Обновление

- Файлы читаются при каждом HTTP запросе к endpoint'у
- Изменения применяются немедленно без перезапуска сервиса
- Для автоматического обновления можно использовать cron:
  ```bash
  # Обновление threat intelligence каждый час
  0 * * * * curl https://threat-feed.example.com/ips.txt > /path/to/threat-intel-ips.txt
  ```

## Безопасность

- Файлы должны находиться в `/app/config/` внутри контейнера
- Используйте volume mount для доступа:
  ```yaml
  volumes:
    - ./config:/app/config:ro  # read-only
  ```
- Максимум 100,000 строк на файл
- Невалидные IP игнорируются с предупреждением в логах

## Примеры источников Threat Intelligence

- [abuse.ch](https://abuse.ch/)
- [Spamhaus DROP](https://www.spamhaus.org/drop/)
- [EmergingThreats](https://rules.emergingthreats.net/)
- Внутренние системы безопасности
- SIEM alerts
