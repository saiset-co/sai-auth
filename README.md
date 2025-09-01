# github.com/saiset-co/sai-auth

Микросервис авторизации и управления доступом для экосистемы SAI, построенный на базе SAI Service Framework.

## Основные возможности

- **Единая точка аутентификации** для всех микросервисов SAI
- **Гибкая система ролей** с наследованием и ограничениями
- **Reference Token** аутентификация с кэшированием в Redis
- **Компиляция разрешений** с поддержкой плейсхолдеров
- **Rate limiting** по пользователям
- **Суперпользователь** с неограниченным доступом
- **REST API** с автоматической документацией

## Быстрый старт

### С Docker Compose (рекомендуется)

```bash
# Клонируем репозиторий
git clone <repository-url>
cd github.com/saiset-co/sai-auth

# Запускаем все сервисы
make up

# Проверяем статус
make status

# Смотрим логи
make logs
```

Сервисы будут доступны на:
- **github.com/saiset-co/sai-auth**: http://localhost:8081
- **SAI-Storage**: http://localhost:8080
- **MongoDB Express**: http://localhost:8082
- **API Documentation**: http://localhost:8081/docs

### Локальная разработка

```bash
# Установка зависимостей
make deps

# Создание конфигурации
make config

# Запуск (требует Redis и SAI-Storage)
make run
```

## Архитектура

### Технологии
- **Framework**: SAI Service Framework (FastHTTP)
- **Database**: MongoDB через SAI-Storage
- **Cache**: Redis для токенов и rate limiting
- **Language**: Go 1.21+

### Структура проекта
```
github.com/saiset-co/sai-auth/
├── cmd/main.go                 # Точка входа
├── internal/
│   ├── auth/providers/         # Auth provider для других сервисов
│   ├── handlers/              # HTTP обработчики
│   ├── middleware/            # Rate limiting middleware
│   ├── models/               # Модели данных
│   ├── repository/           # Интерфейсы репозиториев
│   ├── service/              # Бизнес логика
│   └── storage/              # MongoDB и Redis адаптеры
├── types/                    # Типы конфигурации и запросов
└── docker-compose.yml        # Docker окружение
```

## API Endpoints

### Аутентификация
- `POST /api/v1/auth/login` - Вход в систему
- `POST /api/v1/auth/refresh` - Обновление токена
- `POST /api/v1/auth/logout` - Выход из системы
- `GET /api/v1/roles` - Список ролей
- `POST /api/v1/roles` - Создание роли
- `PUT /api/v1/roles` - Обновление роли
- `DELETE /api/v1/roles` - Удаление роли
- `GET /api/v1/roles/permissions` - Скомпилированные разрешения роли
- `POST /api/v1/roles/permissions` - Тестирование разрешений

## Примеры использования

### Создание пользователя
```bash
curl -X POST http://localhost:8081/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{
    "username": "john.doe",
    "email": "john@example.com", 
    "password": "password123",
    "data": {
      "first_name": "John",
      "last_name": "Doe",
      "department": "IT"
    }
  }'
```

### Создание роли с разрешениями
```bash
curl -X POST http://localhost:8081/api/v1/roles \
  -H "Content-Type: application/json" \
  -d '{
    "name": "content_manager",
    "permissions": [
      {
        "microservice": "sai-storage",
        "method": "GET",
        "path": "/api/v1/documents",
        "rates": [
          {
            "limit": 100,
            "window": "60s"
          }
        ],
        "required_params": [
          {
            "param": "collection",
            "any_value": ["articles", "news"]
          },
          {
            "param": "filter.author_id",
            "value": "$.internal_id"
          }
        ],
        "restricted_params": [
          {
            "param": "collection", 
            "any_value": ["users", "admin_logs"]
          }
        ]
      }
    ]
  }'
```

### Вход в систему
```bash
curl -X POST http://localhost:8081/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "john.doe",
    "password": "password123"
  }'
```

### Проверка токена (для микросервисов)
```bash
curl -X POST http://localhost:8081/api/v1/auth/verify \
  -H "Content-Type: application/json" \
  -d '{
    "token": "your-access-token",
    "microservice": "sai-storage",
    "method": "GET", 
    "path": "/api/v1/documents",
    "request_params": {
      "collection": "articles",
      "filter": {
        "author_id": "user_12345",
        "status": "published"
      }
    }
  }'
```

## Система разрешений

### Структура разрешения
```json
{
  "microservice": "sai-storage",
  "method": "GET",
  "path": "/api/v1/documents",
  "rates": [
    {"limit": 100, "window": "60s"}
  ],
  "required_params": [
    {
      "param": "collection",
      "any_value": ["articles", "news"]
    }
  ],
  "restricted_params": [
    {
      "param": "collection",
      "any_value": ["users", "admin_logs"] 
    }
  ]
}
```

### Типы параметров
- **value: "\*"** - параметр обязателен, любое значение
- **value: "concrete"** - конкретное значение
- **value: "$.field"** - подстановка из профиля пользователя
- **any_value: ["val1", "val2"]** - одно из значений
- **all_values: ["val1", "val2"]** - все значения (для массивов)

### Плейсхолдеры
- `$.internal_id` → ID пользователя
- `$.data.department` → Отдел пользователя
- `$.data.teams` → Команды пользователя (преобразуется в any_value)

### Наследование ролей
- Дочерние роли наследуют разрешения родительских
- Максимальная глубина наследования: 5 уровней
- `restricted_params` имеют приоритет над `required_params`
- Максимум 10 ролей на пользователя
- Максимум 50 разрешений на роль

## Интеграция с другими сервисами

### Auth Provider для SAI Service
```go
// В main.go микросервиса
import "github.com/saiset-co/sai-auth/internal/auth/providers"

func setupAuth() {
    saiAuthProvider := providers.NewSaiAuthProvider("http://github.com/saiset-co/sai-auth:8080")
    
    // Регистрация в SAI Service
    authProvider := sai.AuthProvider()
    authProvider.Register("sai_auth", saiAuthProvider)
}
```

### Конфигурация микросервиса
```yaml
middlewares:
  auth:
    enabled: true
    weight: 60
    params:
      provider: "sai_auth"

auth_providers:
  sai_auth:
    params:
      auth_service_url: "http://github.com/saiset-co/sai-auth:8080"
      timeout: "30s"
```

## Конфигурация

### Переменные окружения
```bash
# Сервер
SERVER_HOST=0.0.0.0
SERVER_PORT=8080

# SAI Storage
STORAGE_URL=http://localhost:8080
STORAGE_USERNAME=
STORAGE_PASSWORD=

# Redis  
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# Аутентификация
ACCESS_TOKEN_TTL=3600s
REFRESH_TOKEN_TTL=86400s
BCRYPT_COST=12
SECRET_KEY=your-secret-key

# Суперпользователь
SUPER_USER_IP_1=127.0.0.1
SUPER_USER_IP_2=::1

# Логирование
LOG_LEVEL=info
```

## Разработка

### Команды Make
```bash
make help          # Список команд
make deps           # Загрузка зависимостей
make build          # Сборка приложения
make run            # Запуск локально
make test           # Тесты
make docker-build   # Сборка Docker образа
make up             # Запуск всех сервисов
make logs           # Просмотр логов
make clean          # Очистка
```

### Требования для разработки
- Go 1.21+
- Docker и Docker Compose
- Make

### Структура тестов
```bash
# Юнит тесты
make test

# Интеграционные тесты  
make test-integration

# Покрытие кода
make test-coverage
```

## Безопасность

### Хеширование паролей
- **bcrypt** с cost 12
- Автоматическое хеширование при создании/обновлении пользователей

### Токены
- **Reference tokens** с хранением в Redis
- Конфигурируемое время жизни access/refresh токенов
- Автоматическая инвалидация при logout

### Суперпользователь
- Первый зарегистрированный пользователь
- Доступ ко всем сервисам без ограничений
- IP whitelist для дополнительной безопасности

### Rate Limiting
- Per-user ограничения на основе разрешений ролей
- Гибкая настройка лимитов через Redis
- Поддержка multiple rate windows

## Мониторинг

### Health Check
```bash
curl http://localhost:8081/health
```

### Метрики
- Встроенные метрики SAI Service
- Доступны на `/metrics` (Prometheus format)
- Memory метрики по умолчанию

### Логирование
- Структурированное логирование через SAI Logger
- Конфигурируемый уровень логирования
- Поддержка console и JSON форматов

## Производительность

### Технические требования (из ТЗ)
- **Время ответа авторизации**: < 50ms (99-й перцентиль)
- **Пропускная способность**: > 1000 RPS на инстансе
- **Кэширование**: Скомпилированные разрешения в Redis

### Оптимизации
- FastHTTP для максимальной производительности
- Redis кэширование токенов и разрешений
- Компиляция разрешений при назначении ролей
- Connection pooling для MongoDB

## Troubleshooting

### Проверка состояния сервисов
```bash
make status
make health
```

### Просмотр логов
```bash
make logs              # Все сервисы
make logs-auth         # Только github.com/saiset-co/sai-auth
make logs-storage      # Только SAI-Storage  
make logs-redis        # Только Redis
```

### Подключение к базам данных
```bash
make mongo-shell       # MongoDB
make redis-cli         # Redis
```

### Сброс окружения
```bash
make dev-reset         # Пересоздание контейнеров
make clean-all         # Полная очистка
```

## Лицензия

MIT License - см. [LICENSE](LICENSE) файл для деталей.

## Поддержка

Для вопросов и проблем создавайте issue в репозитории или обращайтесь к команде разработки./auth/me` - Информация о пользователе
- `POST /api/v1/auth/verify` - Проверка токена (для микросервисов)

### Управление пользователями
- `GET /api/v1/users` - Список пользователей
- `POST /api/v1/users` - Создание пользователя
- `PUT /api/v1/users` - Обновление пользователя
- `DELETE /api/v1/users` - Удаление пользователя
- `POST /api/v1/users/assign-roles` - Назначение ролей
- `POST /api/v1/users/remove-roles` - Удаление ролей

### Управление ролями
- `GET /api/v1/roles` - Список ролей
- `POST /api/v1/roles` - Создание роли
- `PUT /api/v1/roles` - Обновление роли
- `DELETE /api/v1/roles` - Удаление роли
- `GET /api/v1