# DNS Collector - Быстрый старт

## 🚀 Запуск за 5 минут

### Вариант 1: С помощью Docker (рекомендуется)

```bash
# 1. Перейти в директорию
cd /home/test/projects/dns-collector

# 2. Создать директорию для данных
mkdir -p data

# 3. Запустить
docker-compose up -d

# 4. Проверить логи
docker-compose logs -f
```

**Готово!** Сервер слушает на порту 5353 UDP.

---

### Вариант 2: Нативная сборка

```bash
# 1. Перейти в директорию
cd /home/test/projects/dns-collector

# 2. Установить зависимости
go mod download

# 3. Собрать
make build

# 4. Запустить
./build/dns-collector
```

---

## 📤 Отправка тестовых данных

### Способ 1: Python скрипт

```bash
python3 test_client.py
```

### Способ 2: Вручную через netcat

```bash
echo '{"client_ip": "192.168.0.10", "domain": "google.com", "qtype": "A", "rtype": "dns"}' | nc -u -w1 localhost 5353
```

### Способ 3: Отправка нескольких доменов

```bash
# Создайте скрипт
cat > send_domains.sh << 'EOF'
#!/bin/bash
for domain in google.com github.com amazon.com cloudflare.com; do
    echo "{\"client_ip\": \"192.168.0.10\", \"domain\": \"$domain\", \"qtype\": \"A\", \"rtype\": \"dns\"}" | nc -u -w1 localhost 5353
    sleep 0.1
done
EOF

chmod +x send_domains.sh
./send_domains.sh
```

---

## 📊 Проверка работы

### Мониторинг

```bash
# Полная статистика
./monitor.sh

# Или вручную
sqlite3 domains.db "SELECT COUNT(*) as total_domains FROM domain;"
sqlite3 stats.db "SELECT COUNT(*) as total_requests FROM domain_stat;"
```

### Просмотр доменов

```bash
sqlite3 domains.db "SELECT domain, resolv_count, max_resolv FROM domain LIMIT 10;"
```

### Просмотр IP адресов

```bash
sqlite3 domains.db "
SELECT d.domain, i.ip, i.type
FROM domain d
JOIN ip i ON d.id = i.domain_id
LIMIT 20;
"
```

### Просмотр статистики

```bash
sqlite3 stats.db "
SELECT domain, COUNT(*) as count
FROM domain_stat
GROUP BY domain
ORDER BY count DESC
LIMIT 10;
"
```

---

## ⚙️ Настройка

Отредактируйте `config.yaml`:

```yaml
server:
  udp_port: 5353              # Измените порт если нужно

resolver:
  interval_seconds: 300       # Как часто резолвить (в секундах)
  max_resolv: 10             # Сколько раз резолвить каждый домен
  workers: 5                 # Количество параллельных воркеров
```

После изменения конфигурации перезапустите:

```bash
# Docker
docker-compose restart

# Нативно
# Ctrl+C для остановки, затем
./build/dns-collector
```

---

## 🔍 Диагностика

### Проверка что сервер запущен

```bash
# Проверка процесса
ps aux | grep dns-collector

# Проверка порта
sudo netstat -ulnp | grep 5353

# Или с помощью ss
sudo ss -ulnp | grep 5353
```

### Проверка логов

```bash
# Docker
docker-compose logs dns-collector

# systemd (если запущен как сервис)
sudo journalctl -u dns-collector -f
```

### Проверка баз данных

```bash
# Проверка существования
ls -lh *.db

# Проверка структуры
sqlite3 domains.db ".schema"
sqlite3 stats.db ".schema"
```

---

## 🛠️ Частые команды

### Сборка и запуск

```bash
make build          # Собрать
make run            # Собрать и запустить
make clean          # Удалить build артефакты и БД
make test           # Запустить тестовый клиент
```

### Docker команды

```bash
docker-compose up -d              # Запустить в фоне
docker-compose logs -f            # Смотреть логи
docker-compose ps                 # Статус
docker-compose restart            # Перезапустить
docker-compose down               # Остановить и удалить
docker-compose down -v            # Остановить и удалить с volumes
```

### Работа с БД

```bash
# Открыть интерактивную консоль SQLite
sqlite3 domains.db

# Выполнить запрос
sqlite3 domains.db "SELECT * FROM domain LIMIT 5;"

# Экспорт в CSV
sqlite3 -csv -header domains.db "SELECT * FROM domain;" > domains.csv

# Backup
sqlite3 domains.db ".backup domains_backup.db"
```

---

## 📋 Полезные SQL запросы

```bash
# Топ 10 доменов по количеству IP
sqlite3 domains.db "
SELECT d.domain, COUNT(i.id) as ip_count
FROM domain d
LEFT JOIN ip i ON d.id = i.domain_id
GROUP BY d.id
ORDER BY ip_count DESC
LIMIT 10;
"

# Домены которые нужно еще резолвить
sqlite3 domains.db "
SELECT domain, resolv_count, max_resolv
FROM domain
WHERE resolv_count < max_resolv
LIMIT 10;
"

# Статистика за последние 24 часа
sqlite3 stats.db "
SELECT COUNT(*) as requests
FROM domain_stat
WHERE timestamp >= datetime('now', '-24 hours');
"

# Топ клиентов
sqlite3 stats.db "
SELECT client_ip, COUNT(*) as requests
FROM domain_stat
GROUP BY client_ip
ORDER BY requests DESC
LIMIT 10;
"
```

Больше запросов в файле [queries.sql](queries.sql).

---

## 🔄 Обновление

```bash
# Docker
docker-compose down
docker-compose build
docker-compose up -d

# Нативно
make clean
make build
```

---

## 🗑️ Очистка

### Удаление данных

```bash
# Удалить базы данных
rm -f *.db

# Удалить build артефакты
make clean

# Docker: удалить контейнеры и volumes
docker-compose down -v
```

### Очистка старых записей статистики

```bash
# Удалить записи старше 30 дней
sqlite3 stats.db "DELETE FROM domain_stat WHERE timestamp < datetime('now', '-30 days');"

# Оптимизация БД
sqlite3 stats.db "VACUUM;"
```

---

## 📚 Дополнительная информация

- Подробная документация: [README.md](README.md)
- Установка: [INSTALL.md](INSTALL.md)
- Docker: [DOCKER.md](DOCKER.md)
- Архитектура: [ARCHITECTURE.md](ARCHITECTURE.md)
- Обзор: [SUMMARY.md](SUMMARY.md)

---

## ❓ Проблемы?

### Порт занят

```bash
# Найти процесс использующий порт
sudo lsof -i :5353

# Или изменить порт в config.yaml
server:
  udp_port: 5454  # Другой порт
```

### База данных заблокирована

```bash
# Убедитесь что запущен только один экземпляр
ps aux | grep dns-collector

# Убейте дубликаты если есть
killall dns-collector
```

### DNS не резолвится

```bash
# Проверьте DNS сервер системы
cat /etc/resolv.conf

# Проверьте интернет соединение
ping -c 3 8.8.8.8

# Увеличьте таймаут в config.yaml
resolver:
  timeout_seconds: 10
```

---

## 🎯 Быстрый тест всей цепочки

```bash
# 1. Запустить сервер (в отдельном терминале)
./build/dns-collector

# 2. Отправить тестовый запрос
echo '{"client_ip": "192.168.0.10", "domain": "google.com", "qtype": "A", "rtype": "dns"}' | nc -u -w1 localhost 5353

# 3. Подождать несколько секунд

# 4. Проверить что домен добавлен
sqlite3 domains.db "SELECT * FROM domain WHERE domain='google.com';"

# 5. Проверить статистику
sqlite3 stats.db "SELECT * FROM domain_stat WHERE domain='google.com';"

# 6. Подождать время резолвинга (по умолчанию 5 минут, или измените interval_seconds)

# 7. Проверить что IP адреса получены
sqlite3 domains.db "
SELECT d.domain, i.ip, i.type
FROM domain d
JOIN ip i ON d.id = i.domain_id
WHERE d.domain='google.com';
"
```

---

**Совет**: Для ускорения тестирования измените `interval_seconds` на `10` в config.yaml, тогда резолвинг будет происходить каждые 10 секунд.

```yaml
resolver:
  interval_seconds: 10  # Для тестирования
```

Не забудьте вернуть нормальное значение для продакшена!
