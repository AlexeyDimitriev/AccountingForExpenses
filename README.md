# Учёт личных расходов

Приложение для записи расходов по категориям. Можно посмотреть, сколько потрачено за выбранный период, и разобраться, на чём удалось бы сэкономить. Какие расходы считать лишними, пользователь решает сам: автоматической оценки покупок в приложении нет.

В интерфейсе можно создавать, просматривать, изменять и удалять категории и расходы. У расхода есть сумма, дата, категория и необязательный комментарий. Список фильтруется по категории и датам; рядом отображается сумма выбранных расходов. Категорию с расходами удалить нельзя — сначала нужно перенести или удалить связанные записи.

Бэкенд написан на Go, данные хранятся в PostgreSQL. Фронтенд — HTML, CSS и JavaScript без фреймворков. Он обращается к REST API по HTTP и отдаётся тем же сервером. Развёртывание в Kubernetes описано в [отчёте](Отчёт.md).

## Запуск через Docker

Нужны работающий Docker Engine и Docker Compose v2. Команды ниже выполняются из корня проекта. `make` служит сокращением для команд Compose; устанавливать Go и Node.js для этого запуска не нужно. При первой сборке потребуется доступ к сети для загрузки образов и модулей Go.

```sh
cp .env.example .env
make docker-config
make docker-build
make docker-up
```

Без `make` последние три команды выглядят так:

```sh
docker compose config --quiet
docker compose build
docker compose up --detach --wait --wait-timeout 90
```

Откройте [http://localhost:8080](http://localhost:8080). Сначала создайте категорию, затем добавьте расход. Compose запускает PostgreSQL, применяет миграции и поднимает сервер. Завершённый с кодом `0` контейнер `migrate` — ожидаемый результат: он выполняет разовую задачу.

Полезные команды:

```sh
docker compose ps -a
make docker-logs
make docker-down
```

`make docker-down` сохраняет данные в томе PostgreSQL. После следующего `make docker-up` записи останутся на месте. Команда `docker compose down --volumes` удаляет и том с данными, поэтому для обычной остановки она не нужна.

## Запуск в minikube

Нужны Docker, minikube и kubectl. Все команды выполняются из корня проекта.
Манифесты находятся в `k8s/`, ресурсы создаются в стандартном namespace `default`.

### Подготовка образа

```sh
export PATH="$HOME/.local/bin:$PATH"
minikube start --driver=docker --container-runtime=docker --cpus=2 --memory=4096
eval "$(minikube docker-env)"
docker build -t accounting-for-expenses:local .
docker save accounting-for-expenses:local -o /tmp/accounting-for-expenses-image.tar
minikube image load /tmp/accounting-for-expenses-image.tar
eval "$(minikube docker-env -u)"
minikube image ls
```

При сборке через `docker-env` образ уже находится в minikube; сохранение и загрузка
показаны отдельно для выполнения шага `minikube image load` из задания.
Сервер и миграции используют один образ с `imagePullPolicy: IfNotPresent`.

### Развёртывание

```sh
make k8s-up
```

Команда выполняет `scripts/k8s-up.sh`: создаёт настройки и PostgreSQL, ждёт готовности
базы, пересоздаёт Job миграций и ждёт его успешного завершения, затем запускает приложение.
Повторные миграции пропускают уже применённые SQL-файлы. При ошибке миграций скрипт
останавливается и выводит диагностику. Для первого запуска скачивается образ PostgreSQL.

Те же этапы вручную (подходят для записи демонстрации):

```sh
kubectl --context=minikube -n default apply -f k8s/01-config.yaml -f k8s/02-secret.yaml
kubectl --context=minikube -n default apply -f k8s/03-postgres.yaml
kubectl --context=minikube -n default rollout status deployment/postgres --timeout=300s
kubectl --context=minikube -n default delete job expenses-migrate --ignore-not-found --wait=true
kubectl --context=minikube -n default apply -f k8s/04-migrate.yaml
kubectl --context=minikube -n default wait --for=condition=complete job/expenses-migrate --timeout=300s
kubectl --context=minikube -n default apply -f k8s/05-app.yaml
kubectl --context=minikube -n default rollout status deployment/expenses-app --timeout=180s
kubectl --context=minikube -n default get pods,services,pvc
```

В ручном запуске переходите к следующему этапу только после успеха предыдущего.
Один `kubectl apply -f k8s/` не обеспечивает ожидание готовности базы и миграций.
Под Job со статусом `Completed` — нормальный результат.
Пароль `super_secret_password` в `k8s/02-secret.yaml` предназначен для локального учебного запуска.
Смена Secret после инициализации тома не меняет пароль существующей роли PostgreSQL.

### UI и масштабирование

В отдельном терминале запустите и оставьте работающей команду:

```sh
kubectl --context=minikube -n default port-forward service/expenses-app 8080:8080
```

Откройте http://localhost:8080. Создайте категорию, добавьте расход и примените фильтр.
Если порт занят, используйте `8081:8080` и http://localhost:8081.

В другом терминале:

```sh
kubectl --context=minikube -n default scale deployment/expenses-app --replicas=3
kubectl --context=minikube -n default rollout status deployment/expenses-app --timeout=180s
kubectl --context=minikube -n default get pods -o wide
kubectl --context=minikube -n default get deployment expenses-app
```

Ожидается `READY 3/3`. Все реплики используют общую PostgreSQL. Базу оставляем в одном
экземпляре. `port-forward` выбирает один под, поэтому сам по себе не демонстрирует
балансировку между тремя репликами. После удаления выбранного пода его нужно запустить заново.
Повторный `make k8s-up` возвращает число реплик к указанному в YAML значению `1`.

### Обновление, диагностика и остановка

После изменения исходников пересоберите образ по инструкции выше, выполните `make k8s-up`,
а затем перезапустите поды, чтобы они использовали новый образ с тем же тегом:

```sh
kubectl --context=minikube -n default rollout restart deployment/expenses-app
kubectl --context=minikube -n default rollout status deployment/expenses-app --timeout=180s
```

Диагностика:

```sh
kubectl --context=minikube -n default logs -l app=expenses-app --tail=100
kubectl --context=minikube -n default logs job/expenses-migrate
kubectl --context=minikube -n default describe pods -l app=expenses-postgres
kubectl --context=minikube -n default get events --sort-by=.metadata.creationTimestamp
```

Для остановки с сохранением данных используйте `minikube stop`, для продолжения —
`minikube start`. PVC сохраняет данные при пересоздании пода PostgreSQL. Удаление
PVC или всего кластера может удалить данные. Для обычной остановки
эти операции не нужны.

## Настройки

Образец настроек — [.env.example](.env.example). Compose читает `.env` и передаёт значения контейнерам. Сам Go-сервер читает только переменные окружения. Файл `.env` не включается в Git и архив исходников; пароль в примере предназначен для локального учебного запуска.

| Переменная | Назначение и значение по умолчанию |
| --- | --- |
| `APP_VERSION` | Тег образа приложения, `local` |
| `APP_BIND_ADDRESS`, `APP_PORT` | Адрес и порт публикации контейнера, `127.0.0.1:8080` |
| `POSTGRES_USER`, `POSTGRES_DB` | Пользователь и база в Compose, `expenses` |
| `POSTGRES_PASSWORD` | Пароль PostgreSQL; обязателен для Compose |
| `DATABASE_URL` | Строка подключения; в Compose — `host=db port=5432 sslmode=disable` |
| `HTTP_ADDR` | Адрес Go-сервера, `:8080`; внутри Compose зафиксирован на этом значении |
| `LOG_LEVEL` | Уровень журналирования, `INFO` |
| `HTTP_READ_HEADER_TIMEOUT`, `HTTP_READ_TIMEOUT` | Время чтения заголовков и запроса, `5s` и `10s` |
| `HTTP_WRITE_TIMEOUT`, `HTTP_IDLE_TIMEOUT` | Время записи ответа и простоя соединения, `15s` и `60s` |
| `HTTP_REQUEST_TIMEOUT` | Ограничение времени обработки запроса, `10s` |
| `HTTP_SHUTDOWN_TIMEOUT` | Время ожидания активных запросов при остановке, `10s` |
| `DB_CONNECT_TIMEOUT`, `HEALTH_CHECK_TIMEOUT` | Время подключения к БД и проверки её доступности, `5s` и `2s` |
| `CONTAINER_STOP_TIMEOUT` | Сколько Compose ждёт завершения контейнера, `15s` |

В Compose параметры `POSTGRES_USER`, `POSTGRES_PASSWORD` и `POSTGRES_DB` также передаются приложению как `PGUSER`, `PGPASSWORD` и `PGDATABASE`. Драйвер использует их вместе с `DATABASE_URL`. PostgreSQL не публикует порт на хост: к нему обращаются контейнеры во внутренней сети. Настройки `POSTGRES_*` создают пользователя и базу при инициализации пустого тома; смена значений в `.env` не меняет уже созданные роли и базы.

`HTTP_WRITE_TIMEOUT` должен быть больше `HTTP_REQUEST_TIMEOUT`, а `CONTAINER_STOP_TIMEOUT` — больше `HTTP_SHUTDOWN_TIMEOUT`. После изменения настроек выполните `make docker-up`; после изменения исходников сначала выполните `make docker-build`.

## Устройство проекта

| Каталог | Что в нём находится |
| --- | --- |
| `cmd/server` | Сборка зависимостей и запуск HTTP-сервера |
| `cmd/migrate` | Отдельная команда применения миграций |
| `internal/domain` | Категория, расход и правила проверки их данных |
| `internal/application` | Сценарии работы с категориями и расходами, интерфейсы репозиториев |
| `internal/infrastructure` | HTTP-обработчики, PostgreSQL, чтение настроек |
| `migrations` | SQL-схема и встраивание миграций в программу |
| `web` | Страница, стили, JavaScript и тесты денежных расчётов |
| `tests` | Сквозная проверка приложения в Docker |
| `scripts` | Сборка архива исходников |

Зависимости направлены к бизнес-логике: `domain` не знает об HTTP и PostgreSQL, `application` работает через интерфейсы репозиториев, а `infrastructure` реализует эти интерфейсы. Конкретные реализации соединяются в `cmd/server`.

Фронтенд и SQL-миграции встраиваются в бинарные файлы через `go:embed`. Сервер не зависит от расположения исходников при запуске. Миграции учитываются в таблице `schema_migrations`; повторный запуск пропускает уже применённые файлы. Изменять применённую миграцию нельзя: проверка контрольной суммы завершится ошибкой. Для изменения схемы нужно добавить следующий SQL-файл и пересобрать приложение.

## REST API

| Ресурс | Операции |
| --- | --- |
| `/api/categories` | `GET` — список, `POST` — создание |
| `/api/categories/{id}` | `GET` — одна категория, `PUT` — изменение, `DELETE` — удаление |
| `/api/expenses` | `GET` — список с итогом, `POST` — создание |
| `/api/expenses/{id}` | `GET` — один расход, `PUT` — изменение, `DELETE` — удаление |
| `/healthz` | `GET` — процесс отвечает |
| `/readyz` | `GET` — сервер может подключиться к PostgreSQL |

Создание категории:

```sh
curl -i http://localhost:8080/api/categories \
  -H 'Content-Type: application/json' \
  -d '{"name":"Продукты"}'
```

Создание расхода: вместо `1` укажите `id` категории из ответа выше. Сумма передаётся целым числом копеек: `12345` означает 123 рубля 45 копеек.

```sh
curl -i http://localhost:8080/api/expenses \
  -H 'Content-Type: application/json' \
  -d '{"category_id":1,"amount_kopecks":12345,"date":"2026-09-30","comment":"Покупки к ужину"}'

curl 'http://localhost:8080/api/expenses?date_from=2026-09-01&date_to=2026-09-30'
```

Фильтры `category_id`, `date_from` и `date_to` необязательны и могут сочетаться. Обе границы периода включаются в выборку. Ответ списка расходов содержит поля `expenses` и `total_kopecks`. Расходы отсортированы от новых к старым, а при одинаковой дате — по убыванию ID. Дата имеет формат `YYYY-MM-DD` и не содержит времени или часового пояса.

`PUT` принимает те же поля, что и создание, и заменяет данные записи. Успешное создание возвращает `201`, удаление — `204` без тела. Ошибки имеют вид `{"error":{"code":"...","message":"..."}}`: некорректные данные дают `400`, отсутствующая запись — `404`, удаление занятой категории — `409`. Тела запросов ограничены 64 КиБ; неизвестные JSON-поля отклоняются.

## Разработка и тесты

Для работы без контейнера нужны Go 1.26.0 или новее и доступная PostgreSQL с заранее созданными пользователем и базой. Укажите свои параметры подключения:

```sh
export DATABASE_URL='postgres://expenses:expenses_local_password@127.0.0.1:5432/expenses?sslmode=disable'
export HTTP_ADDR='127.0.0.1:8080'
make build
./bin/migrate
./bin/server
```

Этот пример предполагает отдельную PostgreSQL, доступную с хоста: база из стандартного Compose наружу не опубликована.

| Команда | Что проверяет и что требуется |
| --- | --- |
| `make check` | Форматирование, `go vet`, модульные тесты Go и JavaScript; нужны Go и Node.js 18+ |
| `make test-race` | Тесты Go с детектором гонок; нужны CGO и C-компилятор |
| `make test-integration` | Также тесты с настоящей PostgreSQL; нужны Go, Node.js, CGO и `TEST_DATABASE_URL` |
| `make test-e2e` | Полный запуск двух серверов с PostgreSQL; нужны Docker Compose и Python 3.9+ |
| `make cover` | Покрытие тестами Go в `coverage/index.html` |

Для интеграционных тестов используйте тестовую базу. Тесты создают и удаляют отдельные схемы, поэтому пользователь подключения должен иметь право создавать схемы:

```sh
TEST_DATABASE_URL='postgres://expenses:expenses_local_password@127.0.0.1:5432/expenses_test?sslmode=disable' make test-integration
```

Сквозные тесты сами собирают образ и создают отдельный Compose-проект со случайными портами и паролем. Они проверяют CRUD через два экземпляра сервера, фильтры, одновременные записи, повторные миграции, сохранность данных и восстановление после остановки БД. По завершении тестовые контейнеры, сеть и том удаляются. Рабочий проект приложения эти тесты не останавливают.

## Архив для сдачи

```sh
make archive
```

Нужен Python 3.9+. В `dist/AccountingForExpenses.zip` попадут текущие исходники, тесты, настройки сборки, манифесты `k8s/`, этот README и `Отчёт.md`, включая ещё не закоммиченные изменения. `.git`, рабочий `.env`, бинарные файлы и отчёты покрытия не включаются. После распаковки перейдите в каталог `AccountingForExpenses` и следуйте инструкции запуска выше. Скринкаст нужно добавить в архив отдельно перед сдачей; `make archive` пересоздаёт архив и не добавляет видео автоматически.

В приложении пока нет авторизации, отдельных пользователей, лимитов бюджета и пагинации. Все записи составляют один личный бюджет. Для учебного запуска сервер по умолчанию доступен только с локального компьютера.
