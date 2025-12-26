# Unbound Python DNS Forwarder

Python скрипт для интеграции Unbound DNS Resolver (pfSense) с dns-collector сервисом.

## Описание

`python_dns_forwarder.py` — это модуль расширения для Unbound DNS Resolver, который перехватывает DNS-запросы и отправляет информацию о них в сервис dns-collector через UDP.

## Функциональность

Скрипт использует inplace callback механизм Unbound для перехвата DNS-запросов на разных этапах обработки:

- **Resolved queries** (`inplace_reply_callback`) — запросы, разрешенные через внешние DNS-серверы
- **Cached queries** (`inplace_cache_callback`) — запросы, обслуженные из кеша Unbound

### Отправляемые данные

Для каждого DNS-запроса отправляется JSON-сообщение по UDP с полями:

```json
{
  "client_ip": "192.168.1.100",
  "domain": "example.com.",
  "qtype": "A",
  "rtype": "reply"
}
```

| Поле | Описание |
|------|----------|
| `client_ip` | IP-адрес клиента, отправившего запрос |
| `domain` | Доменное имя (FQDN с точкой в конце) |
| `qtype` | Тип DNS-запроса (A, AAAA, CNAME, MX и т.д.) |
| `rtype` | Источник ответа: `reply` (resolved) или `cache` |

## Конфигурация

### Параметры подключения

В начале файла задаются параметры UDP-сервера dns-collector:

```python
UDP_IP = "192.168.0.15"   # IP-адрес dns-collector сервиса
UDP_PORT = 5353            # UDP порт dns-collector
```

**Важно**: Измените эти значения на адрес вашего dns-collector сервиса.

### Фильтрация

Скрипт автоматически игнорирует запросы, исходящие с IP-адреса самого dns-collector (строка 32), чтобы избежать циклических зависимостей.

## Установка в pfSense

1. **Включите Python модуль в Unbound**:
   - Зайдите в `Services > DNS Resolver > Advanced Settings`
   - Включите опцию `Python Module`

2. **Добавьте скрипт**:
   - В разделе `Services > DNS Resolver > Advanced Settings`
   - Найдите поле `Python Module Script`
   - Вставьте содержимое файла `python_dns_forwarder.py`
   - Измените `UDP_IP` и `UDP_PORT` на адрес вашего dns-collector

3. **Перезапустите DNS Resolver**:
   ```bash
   /etc/rc.restart_unbound
   ```

## Проверка работы

### Логи Unbound

Проверьте логи Unbound для сообщений инициализации:

```bash
tail -f /var/log/resolver.log | grep python
```

Должно появиться:
```
python: inited script /usr/local/etc/unbound.pythonmod.py
sock created lazily
```

### Тестирование UDP-отправки

На сервере dns-collector проверьте входящие UDP-пакеты:

```bash
tcpdump -i any -n udp port 5353 -A
```

При выполнении DNS-запроса на pfSense должны появиться JSON-сообщения.

## Совместимость

- **pfSense**: 2.6.0+
- **Unbound**: 1.16.0+ (с поддержкой Python Module)
- **Python**: 2.7 / 3.x (встроенный в pfSense)

## Архитектура

```
┌─────────────┐
│   Клиент    │
└──────┬──────┘
       │ DNS Query
       ▼
┌─────────────────────────┐
│  Unbound DNS Resolver   │
│      (pfSense)          │
│  ┌──────────────────┐   │
│  │ Python Module    │───┼──► UDP JSON → dns-collector:5353
│  │ (этот скрипт)    │   │
│  └──────────────────┘   │
└─────────────────────────┘
       │ DNS Response
       ▼
┌─────────────┐
│   Клиент    │
└─────────────┘
```

## Отключенные callback'и

В скрипте закомментированы (не используются):

- `inplace_query_callback` — перехват исходящих запросов к upstream DNS
- `inplace_local_callback` — перехват ответов из локальных записей

Их можно активировать при необходимости, раскомментировав соответствующие строки в функции `init_standard()`.

## Обработка ошибок

Скрипт устойчив к ошибкам сети:
- Создает UDP-сокет лениво (при первой отправке)
- Логирует ошибки через `log_info()` вместо падения
- Продолжает работу Unbound даже при недоступности dns-collector

## Связанные компоненты

- [dns-collector](../dns-collector/) — сервис сбора и обработки DNS-запросов
- [web-api](../web-api/) — веб-интерфейс для просмотра статистики
