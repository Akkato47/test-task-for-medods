# For reviewers 
Ранее я разрабатывал сервис-аналог ticktick. И там сталкивался с большими проблемами в реализации реккурентности. Нашел стандарт для работы с календарем ICalendar и либы для работы с ним. В go есть пакет ical, который может предложить обширные настройки, но в рамках тестового было решено попробовать реализовать самому. Вероятно есть проблемы

# Task Service

Сервис для управления задачами с HTTP API на Go.

## Требования

- Go `1.23+`
- Docker и Docker Compose

## Быстрый запуск через Docker Compose

```bash
docker compose up --build
```

После запуска сервис будет доступен по адресу `http://localhost:8080`.

Если `postgres` уже запускался ранее со старой схемой — пересоздай volume, чтобы миграции применились заново:

```bash
docker compose down -v && docker compose up --build
```

## Swagger

```text
http://localhost:8080/swagger/
```

OpenAPI JSON: `http://localhost:8080/swagger/openapi.json`

---

## API

### Разовые задачи

**Создать задачу**

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Обход пациентов",
    "description": "Палаты 10-15",
    "status": "new",
    "scheduled_date": "2026-06-15"
  }'
```

**Получить список всех задач**

```bash
curl http://localhost:8080/api/v1/tasks
```

**Получить задачу по ID**

```bash
curl http://localhost:8080/api/v1/tasks/1
```

**Обновить задачу**

```bash
curl -X PUT http://localhost:8080/api/v1/tasks/1 \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Обход пациентов",
    "description": "Палаты 10-20",
    "status": "in_progress"
  }'
```

**Удалить задачу**

```bash
curl -X DELETE http://localhost:8080/api/v1/tasks/1
```

---

### Периодические задачи

Периодическая задача создаётся через тот же `POST /tasks`, но с полем `period`.
В ответе возвращается массив всех сгенерированных экземпляров.

#### Ежедневно (каждые N дней)

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Утренний обзвон пациентов",
    "description": "Уточнить самочувствие",
    "scheduled_date": "2026-06-01",
    "period": {
      "type": "daily",
      "interval": 1,
      "end_date": "2026-06-10"
    }
  }'
```

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Формирование отчётности",
    "scheduled_date": "2026-01-01",
    "period": {
      "type": "daily",
      "interval": 7
    }
  }'
```

#### Ежемесячно (определённое число месяца)

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Инвентаризация оборудования",
    "scheduled_date": "2026-06-01",
    "period": {
      "type": "monthly",
      "day_of_month": 15,
      "end_date": "2026-12-31"
    }
  }'
```

#### На конкретные даты

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Совещание отдела",
    "scheduled_date": "2026-06-05",
    "period": {
      "type": "specific_dates",
      "dates": ["2026-06-05", "2026-06-19", "2026-07-03"]
    }
  }'
```

#### Чётные / нечётные дни месяца

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Проверка журналов",
    "scheduled_date": "2026-06-01",
    "period": {
      "type": "even_odd",
      "parity": "even",
      "end_date": "2026-06-30"
    }
  }'
```

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Проверка журналов",
    "scheduled_date": "2026-06-01",
    "period": {
      "type": "even_odd",
      "parity": "odd",
      "end_date": "2026-06-30"
    }
  }'
```

---

### Фильтрация задач по дате

```bash
curl "http://localhost:8080/api/v1/tasks?from=2026-06-15&to=2026-06-15"

curl "http://localhost:8080/api/v1/tasks?from=2026-06-10&to=2026-06-16"

curl "http://localhost:8080/api/v1/tasks?from=2026-06-01"
```

---

### Обновление периодической задачи (scope)

```bash
curl -X PUT "http://localhost:8080/api/v1/tasks/5?scope=this" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Обзвон пациентов (изменено)",
    "status": "in_progress"
  }'
```

```bash
curl -X PUT "http://localhost:8080/api/v1/tasks/47?scope=this_and_following" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Обзвон пациентов (новое название)",
    "status": "new"
  }'
```

```bash
curl -X PUT "http://localhost:8080/api/v1/tasks/5?scope=all" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Обзвон пациентов (переименовано)",
    "status": "new"
  }'
```

---

### Удаление периодической задачи (scope)

```bash
curl -X DELETE "http://localhost:8080/api/v1/tasks/5?scope=this"
```

```bash
curl -X DELETE "http://localhost:8080/api/v1/tasks/5?scope=this_and_following"
```

```bash
curl -X DELETE "http://localhost:8080/api/v1/tasks/5?scope=all"
```

---