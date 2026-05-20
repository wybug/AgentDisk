GOLANGCI_LINT_VERSION := v1.64.8

.PHONY: build run test clean lint lint-install docker dev-start dev-stop dev-status

APP_NAME := agentdisk

build:
	go build -o bin/$(APP_NAME) .

run:
	go run main.go --config config.yaml

test:
	go test -v -cover ./...

cover:
	go test -coverprofile=coverage.txt ./...
	go tool cover -html=coverage.txt -o coverage.html

clean:
	rm -rf bin/ coverage.txt coverage.html

lint:
	@if ! command -v golangci-lint > /dev/null 2>&1; then \
		echo "golangci-lint is not installed. Run 'make lint-install' or visit https://golangci-lint.run/usage/install/"; \
		exit 1; \
	fi
	golangci-lint run --timeout 5m ./...

lint-install:
	@echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin $(GOLANGCI_LINT_VERSION)

docker:
	docker build -f docker/Dockerfile -t $(APP_NAME):latest .

docker-up:
	docker compose -f docker/docker-compose.yaml up -d

docker-down:
	docker compose -f docker/docker-compose.yaml down

dev-start:
	bash scripts/dev.sh start

dev-stop:
	bash scripts/dev.sh stop

dev-status:
	bash scripts/dev.sh status

sdk-lint:
	cd sdk && ruff check .

sdk-format:
	cd sdk && ruff format --check .

sdk-format-fix:
	cd sdk && ruff format .

sdk-typecheck:
	cd sdk && mypy agentdisk tests

sdk-check: sdk-lint sdk-format sdk-typecheck
	@echo "All SDK checks passed."

web-lint:
	cd web && npm run lint

web-build:
	cd web && npm run build

web-check: web-lint web-build
	@echo "All web checks passed."
