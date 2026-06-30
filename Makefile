GOLANGCI_LINT_VERSION := v1.64.8

.PHONY: build run test clean lint lint-install docker dev-start dev-stop dev-status docs-install docs-dev docs-build

APP_NAME := agentdisk

# FTS5_TAGS turns on the SQLite FTS5 extension for mattn/go-sqlite3. Required
# so /okf/search has a real tokenized index in SQLite mode; without it the
# virtual table CREATE fails at migration and search falls back to no-op.
# The tag must be present on every go build / go test invocation that touches
# the OKF search path.
FTS5_TAGS := fts5

build:
	CGO_ENABLED=1 go build -tags "$(FTS5_TAGS)" -o bin/$(APP_NAME) .

run:
	go run -tags "$(FTS5_TAGS)" main.go --config config.yaml

test:
	go test -tags "$(FTS5_TAGS)" -v -cover ./...

cover:
	go test -tags "$(FTS5_TAGS)" -coverprofile=coverage.txt ./...
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
	cd sdk && mypy src/agentdisk tests

sdk-check: sdk-lint sdk-format sdk-typecheck
	@echo "All SDK checks passed."

web-lint:
	cd web && npm run lint

web-build:
	cd web && npm run build

web-check: web-lint web-build
	@echo "All web checks passed."

docs-install:
	cd docs/site && npm install

docs-dev:
	cd docs/site && npm run docs:dev

docs-build:
	cd docs/site && npm run docs:build
