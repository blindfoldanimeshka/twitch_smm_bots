# Временная почта — TempFastMail

## Что это

TempFastMail — self-hosted сервис одноразовых входящих почтовых ящиков. В стеке
Twitch SMM он запускается **отдельным Docker-контейнером** (`tempfastmail` в
`docker-compose.yml`).

- Исходный код и образ: <https://github.com/kasteckis/TempFastMail> (лицензия **GPL-3.0**).
- Мы запускаем **опубликованный образ `kasteckis/tempfastmail:1.0.0` без изменений**
  (версия закреплена: миграции SQLite/stateful — `:latest` не используем)
  и не копируем его код в этот репозиторий.
- Хранилище — SQLite в named-томе `tempfastmail_data` (`/app/sqlite` в контейнере).
- Письма и ящики автоматически удаляются через `DELETE_EMAILS_AND_INBOXES_OLDER_THAN_HOURS`
  (по умолчанию **48 часов**).

## Важное: сервис сам НЕ принимает SMTP-почту

TempFastMail не слушает 25-порт и не является почтовым сервером. Чтобы письма
действительно доходили до инстанса, нужна цепочка доставки через Cloudflare:

1. Домен на Cloudflare, включённый **Email Routing** с catch-all правилом
   (все письма на домене пересылаются в Worker).
2. Cloudflare Worker **`kasteckis/cloudflare-email-worker-html-parser`**, который
   публикует полученные письма в наш инстанс по HTTP API.
3. Worker авторизуется значением `CREATE_RECEIVED_EMAIL_API_AUTHORIZATION_KEY`
   (наш `.env` — `TEMPFASTMAIL_API_KEY`).

Полная инструкция по настройке:
<https://github.com/kasteckis/tempfastmail/blob/master/docs/CLOUDFLARE_EMAIL_WORKER_SETUP.md>

## Сетевой доступ

По умолчанию контейнер биндится только на loopback:

```yaml
ports:
  - "${TEMPFASTMAIL_BIND:-127.0.0.1}:${TEMPFASTMAIL_PORT:-8090}:80"
```

То есть сервис доступен лишь как `http://localhost:8090` на хосте. Для приёма
реальных входящих писем (Worker должен достучаться до инстанса из интернета)
выставляйте его **только через Cloudflare Tunnel или TLS-обратный прокси**
(nginx/caddy с HTTPS). **Не открывайте 80-й порт хоста напрямую.**

## Первая настройка

- Учётная запись администратора по умолчанию: `admin@admin.dev` / `admin`.
  **Смените пароль сразу после первого входа.**
- Переменные (`docker-compose.yml` → `.env`, см. `.env.example`):
  - `TEMPFASTMAIL_APP_SECRET` — сгенерировать: `openssl rand -hex 32`, не коммитить.
  - `TEMPFASTMAIL_API_KEY` — общий секрет с Cloudflare Worker (в настройках Worker).
  - `TEMPFASTMAIL_URI` / `VITE_TEMP_MAIL_URL` — публичный URL сервиса
    (фронтенд обращается к API почты по `VITE_TEMP_MAIL_URL`).
- `VITE_TEMP_MAIL_URL` прошивается в бандл фронтенда на этапе сборки: после изменения
  требуется пересборка — `docker compose build frontend`.
  - `TEMPFASTMAIL_RETENTION_HOURS` — срок хранения писем.
  - `TEMPFASTMAIL_BIND` / `TEMPFASTMAIL_PORT` — привязка на хосте.

## Безопасность

- Держите `TEMPFASTMAIL_API_KEY` в секрете: кто знает ключ, тот может писать
  письма в любой ящик через API. Не светите его в логах и фронтенде.
- Задайте сильный `APP_SECRET` (иначе сессии и CSRF-защита Symfony предсказуемы).
- **Не выставляйте админ-UI в интернет без дополнительной авторизации**
  (tunnel с access-policy, basic-auth прокси или хотя бы смена пароля админа
  и закрытая регистрация).
- Ограничьте ретеншен (`TEMPFASTMAIL_RETENTION_HOURS`) — не храните письма
  дольше, чем нужно; в БД не должно быть никаких секретов.
- Реальные секреты не коммитить: `.env` вне git, в `.env.example` только пустые
  значения/заглушки.

## Назначение и ограничения использования

> **Не используйте этот сервис для обхода верификации сторонних платформ**
> (Twitch и любых других — это прямое нарушение их правил).

Сервис предназначен **только для собственных тестовых писем** проекта:
проверка собственных рассылок, регистрационных писем нашего стенда,
e2e-тестов потока «письмо → ссылка/код».

## Команды

```bash
# запустить
docker compose up -d tempfastmail

# логи
docker compose logs -f tempfastmail

# UI
http://localhost:8090
```
