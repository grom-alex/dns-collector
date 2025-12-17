# Docker развертывание DNS Collector

## Требования

- Docker 20.10+
- Docker Compose 1.29+

## Быстрый старт

### 1. Сборка образа

```bash
docker build -t dns-collector:latest .
```

### 2. Запуск с Docker Compose

```bash
# Создайте директорию для данных
mkdir -p data

# Запуск
docker-compose up -d

# Просмотр логов
docker-compose logs -f

# Остановка
docker-compose down
```

### 3. Запуск без Docker Compose

```bash
# Создайте директорию для данных
mkdir -p data

# Запуск контейнера
docker run -d \
  --name dns-collector \
  -p 5353:5353/udp \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  --restart unless-stopped \
  dns-collector:latest

# Просмотр логов
docker logs -f dns-collector

# Остановка
docker stop dns-collector
docker rm dns-collector
```

## Конфигурация для Docker

Измените пути к базам данных в `config.yaml`:

```yaml
database:
  domains_db: "/app/data/domains.db"
  stats_db: "/app/data/stats.db"
```

## Тестирование

### Отправка тестового UDP сообщения

```bash
# С хоста
echo '{"client_ip": "192.168.0.10", "domain": "google.com", "qtype": "A", "rtype": "dns"}' | \
  nc -u -w1 localhost 5353

# Или с помощью Python
python3 test_client.py
```

### Проверка работы

```bash
# Проверка логов
docker-compose logs dns-collector

# Проверка баз данных
docker exec dns-collector sqlite3 /app/data/domains.db "SELECT COUNT(*) FROM domain;"
docker exec dns-collector sqlite3 /app/data/stats.db "SELECT COUNT(*) FROM domain_stat;"
```

## Мониторинг

### Просмотр статистики

```bash
# Войти в контейнер
docker exec -it dns-collector /bin/sh

# Внутри контейнера
cd /app/data
sqlite3 domains.db "SELECT * FROM domain LIMIT 10;"
```

### Health Check

```bash
# Проверка состояния
docker inspect --format='{{json .State.Health}}' dns-collector | jq
```

## Backup баз данных

```bash
# Backup
docker exec dns-collector sqlite3 /app/data/domains.db ".backup /app/data/domains.db.backup"
docker exec dns-collector sqlite3 /app/data/stats.db ".backup /app/data/stats.db.backup"

# Копирование на хост
docker cp dns-collector:/app/data/domains.db.backup ./backups/
docker cp dns-collector:/app/data/stats.db.backup ./backups/
```

## Автоматический backup с cron

Создайте скрипт `backup.sh`:

```bash
#!/bin/bash
BACKUP_DIR="./backups/$(date +%Y%m%d)"
mkdir -p "$BACKUP_DIR"

docker exec dns-collector sqlite3 /app/data/domains.db ".backup /app/data/domains.db.backup"
docker exec dns-collector sqlite3 /app/data/stats.db ".backup /app/data/stats.db.backup"

docker cp dns-collector:/app/data/domains.db.backup "$BACKUP_DIR/"
docker cp dns-collector:/app/data/stats.db.backup "$BACKUP_DIR/"

# Удаление старых бэкапов (старше 30 дней)
find ./backups -type d -mtime +30 -exec rm -rf {} +

echo "Backup completed: $BACKUP_DIR"
```

Добавьте в crontab:
```bash
# Бэкап каждый день в 2:00
0 2 * * * /path/to/backup.sh >> /var/log/dns-collector-backup.log 2>&1
```

## Обновление

```bash
# Остановка
docker-compose down

# Пересборка
docker build -t dns-collector:latest .

# Запуск
docker-compose up -d
```

## Troubleshooting

### Контейнер не запускается

```bash
# Проверка логов
docker logs dns-collector

# Проверка конфигурации
docker run --rm -v $(pwd)/config.yaml:/app/config.yaml dns-collector:latest cat /app/config.yaml
```

### Порт занят

```bash
# Проверка, что порт свободен
sudo netstat -tulpn | grep 5353

# Или измените порт в docker-compose.yml
ports:
  - "5454:5353/udp"  # Внешний порт 5454
```

### Проблемы с правами доступа

```bash
# Проверка владельца директории data
ls -la data/

# Изменение владельца
sudo chown -R $(id -u):$(id -g) data/
```

## Производительность

### Ограничение ресурсов

В `docker-compose.yml` можно настроить:

```yaml
deploy:
  resources:
    limits:
      cpus: '2.0'      # Максимум 2 CPU
      memory: 1G       # Максимум 1GB RAM
    reservations:
      cpus: '0.5'      # Минимум 0.5 CPU
      memory: 256M     # Минимум 256MB RAM
```

### Мониторинг ресурсов

```bash
# Статистика контейнера
docker stats dns-collector

# Детальная информация
docker inspect dns-collector
```

## Multi-platform сборка

Для сборки образа под разные архитектуры (amd64, arm64):

```bash
# Создание buildx builder
docker buildx create --name multiarch --use

# Сборка и push для нескольких платформ
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t yourusername/dns-collector:latest \
  --push \
  .
```

## Интеграция с Docker Swarm

Для развертывания в Docker Swarm создайте `docker-stack.yml`:

```yaml
version: '3.8'

services:
  dns-collector:
    image: dns-collector:latest
    ports:
      - "5353:5353/udp"
    volumes:
      - dns-data:/app/data
    configs:
      - source: dns-config
        target: /app/config.yaml
    deploy:
      replicas: 3
      restart_policy:
        condition: on-failure
      resources:
        limits:
          cpus: '1.0'
          memory: 512M

volumes:
  dns-data:

configs:
  dns-config:
    file: ./config.yaml
```

Развертывание:
```bash
docker stack deploy -c docker-stack.yml dns-collector-stack
```

## Kubernetes (опционально)

Пример deployment для Kubernetes:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dns-collector
spec:
  replicas: 2
  selector:
    matchLabels:
      app: dns-collector
  template:
    metadata:
      labels:
        app: dns-collector
    spec:
      containers:
      - name: dns-collector
        image: dns-collector:latest
        ports:
        - containerPort: 5353
          protocol: UDP
        volumeMounts:
        - name: config
          mountPath: /app/config.yaml
          subPath: config.yaml
        - name: data
          mountPath: /app/data
      volumes:
      - name: config
        configMap:
          name: dns-collector-config
      - name: data
        persistentVolumeClaim:
          claimName: dns-collector-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: dns-collector-service
spec:
  type: LoadBalancer
  ports:
  - port: 5353
    protocol: UDP
    targetPort: 5353
  selector:
    app: dns-collector
```
