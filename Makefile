.PHONY: *

include .env
export $(shell sed 's/=.*//' .env)

ENV_VARS = \
	TELEGRAM_BOT_TOKEN=$$KUMOTE_TELEGRAM_BOT_TOKEN \
	TELEGRAM_CHAT_ID=$$KUMOTE_TELEGRAM_CHAT_ID \
	TELEGRAM_ALLOWED_USER_IDS=$$TELEGRAM_ALLOWED_USER_IDS \
	PROJECTS_PATH=$$PROJECTS_PATH \
	CLAUDE_CODE_PATH=$$CLAUDE_CODE_PATH \
	PROJECT_INDEX_PATH=$$PROJECT_INDEX_PATH

show-env:
	@echo "KUMOTE_TELEGRAM_BOT_TOKEN=$$KUMOTE_TELEGRAM_BOT_TOKEN"

run:
	$(ENV_VARS) GIN_MODE=release go run ./cmd/assistant/main.go

dev:
	$(ENV_VARS) $$GOPATH/bin/air

test:
	RUN_INTEGRATION_TESTS=false \
	go test -v -short ./...

test-all:
	TELEGRAM_TEST_BOT_TOKEN=$$KUMOTE_TELEGRAM_BOT_TOKEN \
	TELEGRAM_TEST_CHAT_ID=$$KUMOTE_TELEGRAM_CHAT_ID \
	RUN_INTEGRATION_TESTS=true \
	go test -v ./...

build:
	docker build --no-cache -t kumote-assistant -f ./build/package/assistant/Dockerfile .