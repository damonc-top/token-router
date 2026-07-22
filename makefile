WEB_DIR = ./web
BIN ?= new-api
VERSION ?= $(shell cat VERSION 2>/dev/null)

.PHONY: help build-web build-api build start

help:
	@echo "  make build-web   编译前端 -> web/dist"
	@echo "  make build-api   编译后端 -> ./$(BIN)（需已有 web/dist）"
	@echo "  make build       前端 + 后端"
	@echo "  make start       执行 start-damoncai"

build-web:
	@echo "Building web frontend..."
	@cd $(WEB_DIR) && bun install --frozen-lockfile
	@cd $(WEB_DIR) && DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION='$(VERSION)' bun run build

build-api:
	@if [ ! -f "$(WEB_DIR)/dist/index.html" ]; then \
		echo "missing $(WEB_DIR)/dist — run: make build-web"; \
		exit 1; \
	fi
	@echo "Building Go binary -> ./$(BIN)"
	@go build -o $(BIN) .

build: build-web build-api
	@echo "OK: ./$(BIN)"

start:
	@start-damoncai
