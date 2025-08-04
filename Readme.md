# Archive-Zipper Service

Веб-сервис на Go для скачивания до трёх файлов по URL, упаковки их в ZIP-архив и выдачи пользователю.

---

## 🚀 Быстрый старт

1. Клонировать репозиторий и перейти в папку проекта:

```bash
git clone <repo-url>
cd <project>
```

2. Установить зависимости

```bash
go mod tidy
```

3.Запустить сервер:

```bash
go run main.go --port=8080 --maxTasks=3 --shutdownDelay=3s
```

Или просто воспользоваться командой:

```bash
make dev
```

По умолчанию:

- порт: 8080
- макс. параллельных задач: 3
- таймаут graceful shutdown: 5s

### Эндпоинты

1. Создать задачу
   POST /tasks

Успех:

```json
{
  "task_id": "<UUID>"
}
```

Ошибка:

```json
{
  "error": "server busy"
}
```

Пример curl запроса:

```bash
curl -X POST http://localhost:8080/tasks
```

2. Добавить файл

```bash
POST /tasks/{id}/files
```

Тело запроса:

```json
{
  "url": "https://ru.wikipedia.org/wiki/SunOS#/media/%D0%A4%D0%B0%D0%B9%D0%BB:SunOS_4.1.1_P1270750.jpg"
}
```

Успех:

```json
{
  "message": "file added"
}
```

Возможные ошибки:

invalid request — неверный JSON.

unsupported type — файл не .pdf/.jpeg/.jpg.

max files reached — уже добавлено 3 файла.

task not found — неверный UUID.

Пример curl запроса:

```bash
curl -X POST http://localhost:8080/tasks/<UUID>/files \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com/file.jpg"}'
```

3. Статус задачи

```bash
GET /tasks/{id}
```

Успех (до 3 файлов):

```json
{
  "data": {
    "id": "<UUID>",
    "status": "in_progress",
    "files": [
      { "url": "...", "success": true },
      { "url": "...", "success": false, "error": "timeout" }
    ]
  }
}
```

Успех (после 3х файлов):

```json
{
  "data": {
    "id": "<UUID>",
    "status": "done",
    "files": [
      /* результаты */
    ],
    "archive_url": "http://localhost:8080/tasks/<UUID>/archive"
  }
}
```

Ссылка на сформированный архив содержится в поле archive_url

Пример curl запроса:

```bash
curl http://localhost:8080/tasks/<UUID>
```

### Конфигурация и флаги

--port — порт для HTTP (по умолчанию 8080)

--maxTasks — макс. параллельных задач (по умолчанию 3)

--shutdownDelay — таймаут graceful shutdown (по умолчанию 5s)

### Используемые практики и паттерны

Dependency Injection
— передача TaskManager в HTTP-handlers через handlers.Init().

Graceful Shutdown
— ловля SIGINT/SIGTERM, ожидание завершения активных задач.

Пул задач
— не более N параллельных задач (maxTasks).

Мьютексы и Snapshot-модель
— sync.Mutex для TaskManager и Task; GetSnapshot() возвращает копию без блокировок.

Асинхронная обработка
— архивация запускается в отдельной горутине после 3-го файла.

Валидация и ограничение
— поддерживаются расширения .pdf, .jpeg, .jpg; до 3 файлов на задачу.

Стандартный ZIP
— сборка архива через пакет archive/zip.

Temporary files
— скачанные файлы во временную папку, готовые ZIP в ./archives.

Логирование

- полное логирование происходящего при помощи logrus

### Структура проекта

~/
main.go # точка входа, DI, флаги, graceful shutdown
handlers/ # HTTP-handlers, JSON-утилиты
task/ # бизнес-логика, TaskManager, Task, архивация
archives/ # готовые ZIP-архивы
