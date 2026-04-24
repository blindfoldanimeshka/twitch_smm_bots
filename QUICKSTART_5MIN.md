# ⚡ Быстрый старт за 5 минут

Минимальная инструкция для запуска проекта.

---

## 1️⃣ Установите Docker

Скачайте и установите [Docker Desktop](https://www.docker.com/products/docker-desktop/)

---

## 2️⃣ Получите API ключ для LLM

**Вариант A: OpenRouter (рекомендуется)**
1. Зарегистрируйтесь на [openrouter.ai](https://openrouter.ai/keys)
2. Создайте API ключ (бесплатно)
3. Скопируйте ключ (начинается с `sk-or-v1-`)

**Вариант B: LM Studio (локально)**
1. Скачайте [LM Studio](https://lmstudio.ai/)
2. Загрузите модель (Llama 3.1 8B)
3. Запустите Local Server на порту 1234

---

## 3️⃣ Настройте проект

```bash
# Перейдите в папку проекта
cd C:\Users\blindfold\Desktop\2026\twitchSmm

# Создайте .env файл
copy .env.example .env

# Откройте .env в блокноте и измените:
```

**Для OpenRouter:**
```env
API_KEY=secret-key-12345
LLM_PROVIDER=openrouter
LLM_API_URL=https://openrouter.ai/api/v1
LLM_MODEL=meta-llama/llama-3.1-8b-instruct:free
LLM_API_KEY=sk-or-v1-ваш-ключ
```

**Для LM Studio:**
```env
API_KEY=secret-key-12345
LLM_PROVIDER=lmstudio
LLM_API_URL=http://host.docker.internal:1234/v1
LLM_MODEL=local-model
```

---

## 4️⃣ Запустите проект

```bash
docker compose up -d --build
```

Подождите 2-3 минуты пока все сервисы запустятся.

---

## 5️⃣ Откройте UI и создайте бота

1. Откройте браузер: **http://localhost:3000**

2. Перейдите в **"Настройки"** → добавьте канал:
   - Введите имя канала (без #): `имя_стримера`
   - Нажмите **"Сохранить все настройки"**

3. Перейдите в **"Боты"** → нажмите **"Добавить бота"**:
   - **Twitch Login**: `имя_вашего_бота`
   - **Twitch Password**: `пароль_от_аккаунта`
   - **Имя**: `Любое имя`
   - **Личность**: `веселый_фанат`
   - **Min/Max задержка**: `45` / `180`
   - **Лимит**: `5`
   - Нажмите **"Сохранить"**

4. Нажмите **"Запустить всех"**

---

## ✅ Готово!

Бот подключится к каналу и начнет писать сообщения.

### Проверка:
- Dashboard: http://localhost:3000
- Логи: `docker compose logs backend -f`
- Twitch чат целевого канала

---

## 🛑 Остановка

```bash
docker compose down
```

---

## 📚 Полная документация

Если нужны детали — читайте [QUICKSTART.md](QUICKSTART.md)

---

## ❓ Проблемы?

**Боты не подключаются:**
- Проверьте логин/пароль Twitch
- Используйте пароль, не OAuth токен

**LLM не отвечает:**
- OpenRouter: проверьте API ключ
- LM Studio: убедитесь что сервер запущен на порту 1234

**Сервисы не стартуют:**
```bash
docker compose down
docker compose up -d --build --force-recreate
```
