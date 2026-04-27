ПЛАН ПО ОТЛАДКЕ И ДОВЕДЕНИЮ ДО MVP
АНАЛИЗ ТЕКУЩЕГО СОСТОЯНИЯ ПРОЕКТА
Структура проекта
- Frontend: React + TypeScript + Vite (папка frontend/)
- Backend: Go (папка root уровня)
- API: REST + WebSocket на Go
- База данных: PostgreSQL
- Кэш: Redis (опционально)
---
ЭТАП 1: ТЕКУЩЕЕ ЛОГИРОВАНИЕ
Frontend (существующее)
Компонент	Тип логов	Формат
api/client.ts	Request/Response	console.log с эмодзи
Bots.tsx	CRUD операции	console.log/console.error
Settings.tsx	Сохранение настроек	Toast notifications
useWebSocket.ts	Подключение	console.log/error
Backend (существующее)
Компонент	Тип логов
logic.go	log.Printf() в stdout
IRC клиент	Логи подключения/отправки
API handlers	Базовые ошибки
ВЫЯВЛЕННЫЕ ПРОБЕЛЫ
1. Нет UI логов под каждым ботом - в BotCard отображается только статус, но нет истории сообщений
2. Нет логов в разделах настроек - неизвестно, какие изменения были применены
3. Нет централизованного хранилища логов - всё теряется в консоли браузера
4. WebSocket логи не отображаются в UI - сообщения приходят, но не видны пользователю
---
ЭТАП 2: MVP КРИТЕРИИ
Минимально жизнеспособный продукт
- [ ] Боты работают - запускаются, подключаются к IRC, отправляют сообщения
- [ ] UI управление - создание/редактирование/удаление ботов работает
- [ ] Настройки применяются - каналы, LLM, промпты сохраняются и работают
- [ ] Логи в реальном времени - видно активность ботов в UI
- [ ] Настройки логируются - видно изменения в каждом разделе
Дополнительно для MVP
- [ ] Status bar показывает общее состояние
- [ ] Stats страница (базовая)
- [ ] Dashboard с графиками (базовый)
---
ЭТАП 3: ПЛАН РЕАЛИЗАЦИИ ЛОГОВ В UI
3.1 Компонент BotLogPanel (новый)
frontend/src/components/BotLogPanel.tsx
├── Свойства:
│   ├── botId: string
│   ├── messages: LogMessage[]  (лимит 50-100)
│   └── maxHeight: number
├── UI:
│   ├── Скроллируемый список логов
│   ├── Цветовая кодировка по типу (info/warning/error)
│   ├── Timestamp для каждого сообщения
│   └── Кнопка "Очистить"
3.2 Интеграция в BotCard
BotCard.tsx
├── Добавить:
│   ├── Кнопка "📋 ЛОГИ" внизу карточки
│   └── Click -> раскрывает BotLogPanel под карточкой
└── Состояние:
    └── expandedLogs: boolean
3.3 WebSocket обработка (новый API эндпоинт)
Backend: GET /api/bots/{id}/logs?limit=50
├── Возвращает последние логи бота
├── Хранится в Redis ( если есть ) или PostgreSQL
└── Формат: [{timestamp, type, message, context?}]
3.4 Store для логов (frontend)
frontend/src/stores/logStore.ts (или React Context)
├── Zustand или React Context
├── Хранит логи для каждого бота
├── Методы:
│   ├── addLog(botId, message)
│   ├── getLogs(botId)
│   └── clearLogs(botId)
└── Подписка на WebSocket
3.5 Settings LogPanel
Settings.tsx
├── Каждый раздел получает компонент логов:
│   ├── #channels -> ChannelsLogPanel
│   ├── #debug -> DebugLogPanel  
│   ├── #llm -> LLMConnectionLogPanel
│   ├── #prompts -> PromptsLogPanel
│   └── #advanced -> AdvancedFeaturesLogPanel
├── Компонент SettingsLog.tsx:
│   ├── Применяется к каждому разделу
│   ├── Auto-scroll до последнего лога
│   └── Типы: INFO, SUCCESS, ERROR
└── State:
    └── logs: { [sectionId]: LogEntry[] }
---
ЭТАП 4: РЕАЛИЗАЦИЯ ПО ШАГАМ
Шаг 1: Backend - хранение логов
// internal/api/handlers.go
// Добавить эндпоинт:
func (h *Handler) GetBotLogs(w http.ResponseWriter, r *http.Request)
- SQL: CREATE TABLE bot_logs (...)
- Handler возвращает историю
Шаг 2: Backend - сохранение логов
// logic.go
// В functions:
- runBotLoop: логировать каждое отправленное сообщение
- startBot: логировать запуск
- stopBot: логировать остановку
- applySettings: логировать каждое изменение
Шаг 3: Frontend - Store логов
// frontend/src/stores/logStore.ts
- Создать React Context или использовать Zustand
- Хранить логи локально с лимитом
Шаг 4: BotLogPanel компонент
// frontend/src/components/BotLogPanel.tsx
- Скроллируемый контейнер
- Цвет по типу (info/success/error)
- Timestamp
- Фильтр по типу
Шаг 5: Интеграция в BotCard
// BotCard.tsx изменения:
- Добавить кнопку "📋"
- Состояние expanded: boolean
- Рендер BotLogPanel если expanded
Шаг 6: Settings логи
// Settings.tsx изменения:
- Create useSettingsLogs hook
- Добавить LogPanel компонент в каждый section
- Автоматически логировать изменения
Шаг 7: WebSocket интеграция
// useWebSocket.ts
- Обрабатывать тип сообщения "bot_message"
- Добавлять в store логов
Шаг 8: Тестирование и отладка
Тесты:
- Логи появляются при запуске бота
- Логи появляются при остановке бота
- Логи сохраняются после перезагрузки страницы
- Settings логи показываются в каждом разделе
---
ЭТАП 5: ПРИОРИТЕТЫ РЕАЛИЗАЦИИ
Приоритет	Задача	Сложность
1	Backend - сохранение логов в БД	Средняя
2	API эндпоинт для логов	Низкая
3	Frontend - LogStore	Средняя
4	BotLogPanel компонент	Низкая
5	Интеграция в BotCard	Низкая
6	Settings логи	Средняя
7	WebSocket -> LogStore	Средняя
8	Тестирование	Низкая
---