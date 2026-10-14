# Лабораторная работа №3: Веб-сервис POSPOM

## Описание проекта
Веб-сервис для управления факторами риска в кардиохирургии с REST API, интеграцией с PostgreSQL и MinIO для хранения медиафайлов.

---

## HTTP методы API

### Домен: Пользователь

| № | Метод | URL | Описание | Тело запроса |
|---|-------|-----|----------|--------------|
| 1 | POST | `/api/users/register` | Регистрация нового пользователя | `{"name": "Daria", "email": "daria@test.com", "password": "123456"}` |
| 2 | POST | `/api/users/login` | Аутентификация (заглушка для ЛР 4) | `{"email": "daria@test.com", "password": "123456"}` |
| 3 | POST | `/api/users/logout` | Деавторизация (заглушка для ЛР 4) | — |

### Домен: Услуги (Факторы риска)

| № | Метод | URL | Описание | Тело запроса |
|---|-------|-----|----------|--------------|
| 4 | POST | `/api/risk_factors` | Создание/обновление черновика услуги с загрузкой файлов image и video | Multipart form-data: name, description, age, bmi, risk_percentage, image (file), video (file) |
| 5 | GET | `/api/risk_factors/draft` | Получение черновика текущего пользователя (не более 1 записи) | — |
| 6 | PUT | `/api/risk_factors/:id/publish` | Публикация услуги (смена статуса draft → published). Только для создателя | — |
| 7 | DELETE | `/api/risk_factors/:id` | Мягкое удаление услуги (смена статуса → deleted). Только для создателя | — |
| 8 | POST | `/api/risk_factors/:id/like` | Поставить (action: 1) или убрать (action: 0) лайк | `{"action": 1}` |
| 9 | GET | `/api/risk_factors/feed` | Получение первой опубликованной услуги для ленты | — |
| 10 | GET | `/api/risk_factors?maxAge=N` | Получение списка опубликованных услуг с фильтрацией по возрасту | — |

---

## Структура таблиц БД

### Таблица: `users` (Пользователь)

| Поле | Тип | Описание |
|------|-----|----------|
| id | INTEGER (PK) | Первичный ключ, автоинкремент |
| name | VARCHAR(100) | ФИО пользователя |
| email | VARCHAR(255) | E-mail (уникальный) |
| password | VARCHAR(255) | Пароль |

### Таблица: `risk_factors` (Фактор риска)

| Поле | Тип | Описание |
|------|-----|----------|
| id | INTEGER (PK) | Первичный ключ, автоинкремент |
| name | VARCHAR(255) | Название фактора риска |
| description | VARCHAR(255) | Описание |
| status | VARCHAR(20) | Статус: draft / published / deleted |
| image_url | VARCHAR(500) | Имя файла изображения в MinIO |
| video_url | VARCHAR(500) | Имя файла видео в MinIO |
| age | INTEGER | Возраст пациентов |
| bmi | FLOAT | ИМТ (индекс массы тела) |
| risk_percentage | FLOAT | Процент риска |
| created_at | DATETIME | Дата создания (авто) |
| created_by | INTEGER (FK) | ID создателя → users.id |

### Таблица: `likes` (Лайк)

| Поле | Тип | Описание |
|------|-----|----------|
| id | INTEGER (PK) | Первичный ключ, автоинкремент |
| user_id | INTEGER (FK) | ID пользователя → users.id |
| risk_factor_id | INTEGER (FK) | ID фактора риска → risk_factors.id |
| created_at | DATETIME | Дата создания (авто) |

---

## Связи между таблицами (ER-диаграмма)

- **users** 1:N **risk_factors** — один пользователь может создать много факторов риска (связь через поле `created_by`)
- **users** 1:N **likes** — один пользователь может поставить много лайков
- **risk_factors** 1:N **likes** — один фактор риска может иметь много лайков
- Связь **users ↔ risk_factors** через таблицу **likes** реализует отношение "многие-ко-многим"

---

## Технологический стек

- **Backend:** Go + Gin (веб-фреймворк)
- **ORM:** GORM (работа с PostgreSQL)
- **База данных:** PostgreSQL
- **Хранилище файлов:** MinIO (изображения и видео)
- **API клиент:** Postman / Insomnia

---

## Функция-синглтон для пользователя

В файле `internal/utils/user.go` реализована функция-синглтон, возвращающая ID текущего пользователя как константу:

```go
func GetCurrentUserID() uint {
    return 1 // Заглушка: имитация авторизованного пользователя
}
