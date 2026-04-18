# twitch_smm_bots

Текущее состояние проекта на 18.04.2026.

## О проекте

`twitch_smm_bots` - backend на Go для управления набором Twitch-ботов, которые:
- подключаются к Twitch IRC;
- генерируют короткие сообщения через локальную LLM (LM Studio, OpenAI-compatible API);
- отправляют сообщения в чат с персонализациями и ограничениями по частоте.

Сейчас проект работает как консольное приложение без веб-панели.

## Что реализовано

- Запуск менеджера ботов из `logic.go`.
- Хранение конфигурации ботов в PostgreSQL (`internal/storage/postgres.go`).
- Автосоздание схемы таблицы `bots` и триггера `updated_at`.
- Подключение к Twitch IRC по TLS (`internal/twitch/irc.go`), отправка `PRIVMSG`, обработка `PING/PONG`.
- Интеграция с LM Studio (`internal/llm/lmstudio.go`) через `/chat/completions`.
- Набор базовых personality-профилей:
  - `веселый_фанат`
  - `токсик`
  - `нейтрал`
  - `хайпер`
- Локальные и глобальные ограничения частоты отправки сообщений.
- Консольные команды управления:
  - `list`
  - `start <id>`
  - `stop <id>`
  - `start-all`
  - `stop-all`
  - `quit`
- Docker Compose для локальной инфраструктуры Postgres + pgAdmin.

## Текущая структура

- `logic.go` - entrypoint, менеджер ботов, консольный интерфейс.
- `internal/llm/lmstudio.go` - клиент LM Studio.
- `internal/twitch/irc.go` - IRC-клиент Twitch.
- `internal/storage/postgres.go` - доступ к Postgres и схема данных.
- `docker-compose.yml` - локальный запуск Postgres/pgAdmin.
- `db/init` - SQL-инициализация контейнера Postgres.
- `task.md` - расширенное ТЗ и roadmap.

## Быстрый запуск (локально)

1. Поднять инфраструктуру:
   - `docker compose up -d`
2. Подготовить ботов в таблице `bots` (через SQL).
3. Запустить LM Studio и загрузить локальную модель.
4. Установить переменные окружения:
   - `TWITCH_CHANNEL` (обязательно)
   - `DATABASE_URL` (по умолчанию: `postgres://botfarm:botfarm@localhost:5432/twitch_bot_farm?sslmode=disable`)
   - `LMSTUDIO_URL` (по умолчанию: `http://127.0.0.1:1234/v1`)
   - `LMSTUDIO_MODEL` (по умолчанию: `local-model`)
5. Запустить приложение:
   - `go run logic.go`

## Ограничения на текущий момент

- Нет frontend-панели (React/Next) и API-слоя (REST/gRPC).
- Нет отдельного сервиса оркестрации нескольких нод.
- Нет Redis-кэша и прокси-ротатора.
- Нет полноценных e2e/нагрузочных тестов.
- Нет CI/CD пайплайна в репозитории.

## Статус

Проект находится в активной стадии backend MVP: ядро управления ботами, LLM и Twitch IRC уже собрано, но часть из исходного ТЗ еще не реализована.
