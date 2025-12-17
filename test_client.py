#!/usr/bin/env python3
"""
Простой тестовый клиент для отправки DNS запросов в DNS Collector
"""

import socket
import json
import sys
import time

def send_dns_query(host="localhost", port=5353, domain="google.com", client_ip="192.168.0.10", qtype="A", rtype="dns"):
    """Отправка DNS запроса через UDP"""

    # Создаем JSON сообщение
    message = {
        "client_ip": client_ip,
        "domain": domain,
        "qtype": qtype,
        "rtype": rtype
    }

    # Преобразуем в JSON строку
    json_message = json.dumps(message)

    print(f"Sending: {json_message}")

    # Создаем UDP сокет
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)

    try:
        # Отправляем сообщение
        sock.sendto(json_message.encode('utf-8'), (host, port))
        print(f"Message sent to {host}:{port}")
    except Exception as e:
        print(f"Error sending message: {e}")
    finally:
        sock.close()

def main():
    """Основная функция"""

    # Список тестовых доменов
    test_domains = [
        "google.com",
        "github.com",
        "cloudflare.com",
        "amazon.com",
        "microsoft.com",
    ]

    print("DNS Collector Test Client")
    print("-" * 50)

    # Отправляем запросы для каждого домена
    for i, domain in enumerate(test_domains, 1):
        print(f"\n[{i}/{len(test_domains)}] Testing domain: {domain}")

        # Отправляем IPv4 запрос
        send_dns_query(domain=domain, qtype="A", rtype="dns")

        # Небольшая задержка между запросами
        time.sleep(0.1)

        # Отправляем IPv6 запрос
        send_dns_query(domain=domain, qtype="AAAA", rtype="dns")

        time.sleep(0.1)

    print("\n" + "-" * 50)
    print(f"Sent {len(test_domains) * 2} test queries")
    print("\nCheck the DNS Collector logs to verify reception")

if __name__ == "__main__":
    main()
