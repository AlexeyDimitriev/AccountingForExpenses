.DEFAULT_GOAL := help

GO ?= go
GOFMT ?= gofmt

.PHONY: help test test-race cover fmt vet check

help:
	@printf '%s\n' \
		'make test      — запустить тесты' \
		'make test-race — запустить тесты с детектором гонок (нужны CGO и C-компилятор)' \
		'make cover     — создать отчёт покрытия в coverage/index.html' \
		'make fmt       — отформатировать Go-код' \
		'make vet       — выполнить статический анализ' \
		'make check     — проверить форматирование, выполнить анализ и тесты'

test:
	$(GO) test -count=1 ./...

test-race:
	$(GO) test -race -count=1 ./...

cover:
	mkdir -p coverage
	$(GO) test -count=1 -coverprofile=coverage/coverage.out ./...
	$(GO) tool cover -html=coverage/coverage.out -o coverage/index.html

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

check:
	@unformatted=$$($(GOFMT) -l cmd internal); \
	status=$$?; \
	if [ "$$status" -ne 0 ]; then exit "$$status"; fi; \
	if [ -n "$$unformatted" ]; then \
		printf 'Выполните make fmt для файлов:\n%s\n' "$$unformatted"; \
		exit 1; \
	fi
	$(MAKE) vet
	$(MAKE) test
