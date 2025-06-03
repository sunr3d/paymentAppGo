# GoMicro — Микросервисная система для управления платежами

Пет-проект для изучения микросервисной архитектуры на Go с использованием MySQL и Kubernetes.

## Описание

Система состоит из нескольких микросервисов для управления транзакциями между кошельками, ведения учёта (ledger) и аутентификации пользователей.

## Структура проекта

- `src/gomicro/ledger/`  
  Go-сервис для учёта транзакций и балансов.

- `src/gomicro/money_movement/`  
  Go-сервис для обработки и оркестрации переводов между кошельками.

- `src/gomicro/mysql_auth/`  
  Go-сервис и SQL-скрипты для аутентификации пользователей и управления доступом.

- `src/gomicro/mysql_ledger/`  
  MySQL-схемы, инициализация и манифесты для сервиса учёта.

- `src/gomicro/mysql_money_movement/`  
  MySQL-схемы, инициализация и манифесты для сервиса переводов.

## Технологии

- Go (Golang)
- MySQL
- Kafka (sarama)
- JWT (JSON Web token)
- gRPC
- Kubernetes (манифесты YAML)
- Docker

## Быстрый старт

1. Клонируйте репозиторий.
2. Примените Kubernetes-манифесты из папок `manifests/`.
3. Соберите и запустите Go-сервисы в соответствующих папках.
4. Инициализируйте базы данных с помощью предоставленных SQL-скриптов.

## Пример запроса

Пример JSON для авторизации транзакции (`src/gomicro/test_jsons/authorize_payload.json`):

```json
{
  "customer_wallet_user_id": "sunr3d.coding@gmail.com",
  "merchant_wallet_user_id": "merchant_id",
  "cents": 1000,
  "currency": "USD"
}