.DEFAULT_GOAL := help

GO ?= go
GOFMT ?= gofmt
NODE ?= node
COMPOSE ?= docker compose
PYTHON ?= python3

.PHONY: help run build test test-web test-race test-integration test-e2e migrate cover fmt vet check
.PHONY: docker-build docker-up docker-down docker-migrate docker-logs docker-config
.PHONY: archive

help:
	@printf '%s\n' \
		'make run       — запустить сервер (нужен DATABASE_URL)' \
		'make build     — собрать сервер и миграции в bin/' \
		'make docker-build — собрать образ приложения (сначала скопируйте .env.example в .env)' \
		'make docker-up — запустить окружение и дождаться готовности' \
		'make docker-down — остановить окружение, сохранив данные БД' \
		'make docker-migrate — запустить миграции отдельным контейнером' \
		'make docker-logs — показать логи окружения' \
		'make docker-config — проверить конфигурацию Compose' \
		'make test      — запустить тесты' \
		'make test-web  — проверить денежные расчёты интерфейса (нужен Node.js 18+)' \
		'make test-race — запустить тесты с детектором гонок (нужны CGO и C-компилятор)' \
		'make test-integration — проверить PostgreSQL (нужен TEST_DATABASE_URL; создаются временные схемы)' \
		'make test-e2e  — сквозные тесты двух серверов в Docker (нужны Docker Compose и Python 3.9+)' \
		'make migrate   — применить SQL-миграции (нужен DATABASE_URL)' \
		'make cover     — создать отчёт покрытия в coverage/index.html' \
		'make archive   — собрать исходники и отчёт в dist/AccountingForExpenses.zip (нужен Python 3.9+)' \
		'make fmt       — отформатировать Go-код' \
		'make vet       — выполнить статический анализ' \
		'make check     — проверить форматирование, выполнить анализ и тесты'

run:
	$(GO) run ./cmd/server

archive:
	$(PYTHON) scripts/archive.py

docker-build:
	$(COMPOSE) build

docker-up:
	$(COMPOSE) up --detach --wait --wait-timeout 90

docker-down:
	$(COMPOSE) down

docker-migrate:
	$(COMPOSE) run --rm migrate

docker-logs:
	$(COMPOSE) logs --follow

docker-config:
	$(COMPOSE) config --quiet

build:
	mkdir -p bin
	$(GO) build -o bin/server ./cmd/server
	$(GO) build -o bin/migrate ./cmd/migrate

test: test-web
	$(GO) test -count=1 ./...

test-web:
	$(NODE) --test web/money_test.mjs

test-e2e:
	$(PYTHON) tests/e2e.py --compose "$(COMPOSE)"

test-race:
	$(GO) test -race -count=1 ./...

test-integration:
	@test -n "$$TEST_DATABASE_URL" || { printf '%s\n' 'Задайте TEST_DATABASE_URL для тестовой PostgreSQL'; exit 1; }
	$(MAKE) test-web
	$(GO) test -tags=integration -race -count=1 -timeout=120s ./...

migrate:
	@test -n "$$DATABASE_URL" || { printf '%s\n' 'Задайте DATABASE_URL'; exit 1; }
	$(GO) run ./cmd/migrate

cover:
	mkdir -p coverage
	$(GO) test -count=1 -coverprofile=coverage/coverage.out ./...
	$(GO) tool cover -html=coverage/coverage.out -o coverage/index.html

fmt:
	$(GOFMT) -w cmd internal migrations web

vet:
	$(GO) vet ./...

check:
	@unformatted=$$($(GOFMT) -l cmd internal migrations web); \
	status=$$?; \
	if [ "$$status" -ne 0 ]; then exit "$$status"; fi; \
	if [ -n "$$unformatted" ]; then \
		printf 'Выполните make fmt для файлов:\n%s\n' "$$unformatted"; \
		exit 1; \
	fi
	$(MAKE) vet
	$(MAKE) test
