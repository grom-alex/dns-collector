# Инструкция по установке и запуску DNS Collector

## Требования

- Go 1.21 или выше
- SQLite3

## Установка Go (если не установлен)

### Ubuntu/Debian
```bash
sudo apt update
sudo apt install golang-go
```

### CentOS/RHEL
```bash
sudo yum install golang
```

### Или скачайте последнюю версию с официального сайта
```bash
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

## Сборка и запуск

### 1. Переход в директорию проекта
```bash
cd /home/test/projects/dns-collector
```

### 2. Установка зависимостей
```bash
go mod download
go mod tidy
```

### 3. Сборка программы
```bash
# Простая сборка
make build

# Или напрямую через Go
go build -o dns-collector ./cmd/dns-collector
```

### 4. Настройка конфигурации
Отредактируйте файл `config.yaml` под свои нужды:
```bash
nano config.yaml
```

### 5. Запуск программы
```bash
# С конфигом по умолчанию
./build/dns-collector

# Или с указанием пути к конфигу
./build/dns-collector -config /path/to/config.yaml
```

## Тестирование

### Тест 1: Отправка через netcat (Linux/macOS)
```bash
echo '{"client_ip": "192.168.0.10", "domain": "google.com", "qtype": "A", "rtype": "dns"}' | nc -u -w1 localhost 5353
```

### Тест 2: Python скрипт
```bash
python3 test_client.py
```

### Тест 3: Проверка базы данных
```bash
# Проверка таблицы доменов
sqlite3 domains.db "SELECT * FROM domain;"

# Проверка IP адресов
sqlite3 domains.db "SELECT d.domain, i.ip, i.type FROM domain d JOIN ip i ON d.id = i.domain_id;"

# Проверка статистики
sqlite3 stats.db "SELECT * FROM domain_stat ORDER BY timestamp DESC LIMIT 10;"
```

## Запуск как системный сервис (systemd)

Создайте файл `/etc/systemd/system/dns-collector.service`:

```ini
[Unit]
Description=DNS Collector Service
After=network.target

[Service]
Type=simple
User=your-user
WorkingDirectory=/home/test/projects/dns-collector
ExecStart=/home/test/projects/dns-collector/build/dns-collector -config /home/test/projects/dns-collector/config.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Затем:
```bash
sudo systemctl daemon-reload
sudo systemctl enable dns-collector
sudo systemctl start dns-collector
sudo systemctl status dns-collector
```

## Мониторинг логов

```bash
# Если запущен как сервис
sudo journalctl -u dns-collector -f

# Если запущен вручную - смотрите вывод в терминале
```

## Устранение неполадок

### Порт занят
Если UDP порт занят, измените `udp_port` в config.yaml

### База данных заблокирована
Убедитесь, что не запущено несколько экземпляров программы

### DNS резолвинг не работает
Проверьте настройки DNS сервера в системе:
```bash
cat /etc/resolv.conf
```

## Производительность

- Программа использует пул воркеров для параллельного резолвинга
- Количество воркеров настраивается в `config.yaml` (параметр `workers`)
- Для высоконагруженных систем рекомендуется:
  - Увеличить количество воркеров (5-20)
  - Уменьшить interval_seconds для более частого резолвинга
  - Использовать SSD для баз данных
