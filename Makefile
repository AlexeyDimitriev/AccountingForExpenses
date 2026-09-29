.DEFAULT_GOAL := help

GO ?= go
GOFMT ?= gofmt

.PHONY: help run build test test-race test-integration migrate cover fmt vet check

help:
	@printf '%s\n' \
		'make run       — запустить сервер (нужен DATABASE_URL)' \
		'make build     — собрать сервер и миграции в bin/' \
		'make test      — запустить тесты' \
		'make test-race — запустить тесты с детектором гонок (нужны CGO и C-компилятор)' \
		'make test-integration — проверить PostgreSQL (нужен TEST_DATABASE_URL; создаются временные схемы)' \
		'make migrate   — применить SQL-миграции (нужен DATABASE_URL)' \
		'make cover     — создать отчёт покрытия в coverage/index.html' \
		'make fmt       — отформатировать Go-код' \
		'make vet       — выполнить статический анализ' \
		'make check     — проверить форматирование, выполнить анализ и тесты'

run:
	$(GO) run ./cmd/server

build:
	mkdir -p bin
	$(GO) build -o bin/server ./cmd/server
	$(GO) build -o bin/migrate ./cmd/migrate

test:
	$(GO) test -count=1 ./...

test-race:
	$(GO) test -race -count=1 ./...

test-integration:
	@test -n "$$TEST_DATABASE_URL" || { printf '%s\n' 'Задайте TEST_DATABASE_URL для тестовой PostgreSQL'; exit 1; }
	$(GO) test -tags=integration -race -count=1 -timeout=120s ./...

migrate:
	@test -n "$$DATABASE_URL" || { printf '%s\n' 'Задайте DATABASE_URL'; exit 1; }
	$(GO) run ./cmd/migrate

cover:
	mkdir -p coverage
	$(GO) test -count=1 -coverprofile=coverage/coverage.out ./...
	$(GO) tool cover -html=coverage/coverage.out -o coverage/index.html

fmt:
	$(GOFMT) -w cmd internal migrations

vet:
	$(GO) vet ./...

check:
	@unformatted=$$($(GOFMT) -l cmd internal migrations); \
	status=$$?; \
	if [ "$$status" -ne 0 ]; then exit "$$status"; fi; \
	if [ -n "$$unformatted" ]; then \
		printf 'Выполните make fmt для файлов:\n%s\n' "$$unformatted"; \
		exit 1; \
	fi
	$(MAKE) vet
	$(MAKE) test
