include .env
export $(shell sed 's/=.*//' .env)

show-env:
	@echo "KUMOTE_TELEGRAM_BOT_TOKEN=$$KUMOTE_TELEGRAM_TEST_BOT_TOKEN"

test:
	RUN_INTEGRATION_TESTS=false \
	go test -v -short ./...

test-all:
	TELEGRAM_TEST_BOT_TOKEN=$$KUMOTE_TELEGRAM_TEST_BOT_TOKEN \
	TELEGRAM_TEST_CHAT_ID=$$KUMOTE_TELEGRAM_TEST_CHAT_ID \
	RUN_INTEGRATION_TESTS=true \
	go test -v ./...