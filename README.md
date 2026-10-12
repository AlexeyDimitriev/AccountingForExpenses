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

## VictoriaLogs

VictoriaLogs single-node разворачивается в том же minikube. Манифест
`k8s/06-victoria-logs.yaml` создаёт Deployment, Service и PVC на 2 GiB.
Данные хранятся в `/victoria-logs-data`, срок хранения составляет 7 дней.

```sh
kubectl --context=minikube -n default apply -f k8s/06-victoria-logs.yaml
kubectl --context=minikube -n default rollout status deployment/victoria-logs --timeout=300s
kubectl --context=minikube -n default port-forward service/victoria-logs 9428:9428
```

Оставьте последнюю команду работающей и откройте http://localhost:9428/select/vmui/.
Внутри namespace `default` сервис доступен по адресу `http://victoria-logs:9428`.
Проверка готовности использует `/health`. При пересоздании пода данные сохраняются
на PVC. После пересоздания пода команду `port-forward` нужно запустить заново.

### Доставка логов через Fluent Bit

Манифест `k8s/07-fluent-bit.yaml` содержит ConfigMap и DaemonSet. В однодовом
minikube работает один сборщик. Он читает stdout контейнеров `expenses-app` и
`expenses-migrate` в namespace `default`. Конфигурация рассчитана на Docker runtime.

```sh
kubectl --context=minikube -n default apply -f k8s/07-fluent-bit.yaml
kubectl --context=minikube -n default rollout status daemonset/expenses-fluent-bit --timeout=300s
kubectl --context=minikube -n default logs -l app=expenses-fluent-bit --tail=30
```

Fluent Bit читает файлы через `/var/log/containers`. Каталоги `/var/log` и
`/var/lib/docker/containers` подключены с узла только для чтения, поскольку файлы
контейнеров доступны через символические ссылки. Сборщик работает от root для чтения
этих файлов, но не использует privileged-режим или доступ к Kubernetes API.

Первый парсер разбирает оболочку Docker, второй разбирает JSON приложения из поля
`log`. Записи без `timestamp` пропускаются. Поля HTTP-запросов сохраняются отдельно.
Fluent Bit отправляет записи через HTTP `/insert/jsonline`. VictoriaLogs использует
`timestamp` как `_time`, `message` как `_msg`, а `service.name` как поле потока.
Поле `log_file` позволяет определить под, из которого пришла запись.

Позиции чтения и очередь отправки находятся на узле в `/var/lib/fluent-bit-expenses`.
При пересоздании сборщика чтение продолжается с сохранённой позиции. Для новых файлов
оно начинается с начала. Очередь ограничена 128 MiB, неудачные отправки повторяются.
При длительной недоступности VictoriaLogs и переполнении очереди возможна потеря
старых записей. Доставка не гарантирует отсутствие дублей при повторных отправках.

Для проверки в vmui выполните:

```text
_time:15m service.name:="expenses-app"
```

После изменения ConfigMap перезапустите сборщик:

```sh
kubectl --context=minikube -n default rollout restart daemonset/expenses-fluent-bit
kubectl --context=minikube -n default rollout status daemonset/expenses-fluent-bit --timeout=180s
```

## Алерт на ошибки приложения

Манифест `k8s/08-vmalert.yaml` запускает vmalert и содержит правило
`ExpensesApplicationErrors`. Каждые 15 секунд оно проверяет, есть ли записи
`ERROR` сервиса `expenses-app` за последние 5 минут. Условие и аннотации
находятся в ConfigMap этого манифеста.

```sh
kubectl --context=minikube -n default apply -f k8s/08-vmalert.yaml
kubectl --context=minikube -n default rollout status deployment/vmalert --timeout=180s
kubectl --context=minikube -n default port-forward service/vmalert 8880:8880
```

Интерфейс доступен по адресу http://localhost:8880/vmalert/.
`Firing` означает наличие ошибок в выбранном окне, `Inactive` означает,
что условие не выполняется. Дополнительной задержки `for` нет.
После последней ошибки алерт снимается примерно через 5 минут, с учётом
интервала проверки. Доставка логов тоже занимает некоторое время.

Режим `-notifier.blackhole` позволяет наблюдать состояние в интерфейсе,
без внешних уведомлений. История состояний не сохраняется в отдельном
хранилище. После перезапуска vmalert заново вычисляет состояние по логам.
После изменения ConfigMap перезапустите Deployment `vmalert`.

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

## Логи приложения

Сервер и миграции пишут по одной JSON-записи на строку в stdout. Общий логгер
настроен в `internal/infrastructure/logging`. Каждая запись содержит `timestamp`
в формате RFC3339 с часовым поясом, `level`, `message` и `service.name`.
Имя сервиса равно `expenses-app` для сервера и `expenses-migrate` для миграций.

Записи HTTP-запросов также содержат `method`, `path`, `status`, `bytes` и
`duration_ms`. Длительность задана числом в миллисекундах с дробной частью.
Параметры строки запроса и тела запросов в журнал не записываются.
Успешные проверки `/healthz` и `/readyz` записываются на уровне `DEBUG`,
ошибки сервера и паники на уровне `ERROR`, остальные запросы на уровне `INFO`.

После успешного создания, изменения и удаления категории или расхода сервер
записывает событие с понятным сообщением. Поле `event` содержит `category.created`,
`category.updated`, `category.deleted`, `expense.created`, `expense.updated` или
`expense.deleted`. Поля `category_id` и `expense_id` указывают идентификатор
соответствующего объекта. События создания и удаления категории содержат
`category_name`, а изменения категории содержат `old_name` и `new_name`.

При создании и удалении расхода записываются `category_id`, `category_name`,
`amount_kopecks` (целое число копеек) и `date` в формате `YYYY-MM-DD`.
При изменении расхода только изменённые поля получают префиксы `old_` и `new_`.
Смена категории включает её идентификатор и название до и после изменения.
Изменение комментария отмечается флагом `comment_changed`, сам текст не записывается.
Значения до изменения и удаления читаются перед выполнением операции.
События предназначены для диагностики и не являются транзакционным журналом аудита.

Уровень сервера задаётся через `LOG_LEVEL`, миграции используют уровень `INFO`.

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
