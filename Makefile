.PHONY: build run test clean lint docker

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
	golangci-lint run ./...

docker:
	docker build -f docker/Dockerfile -t $(APP_NAME):latest .

docker-up:
	docker compose -f docker/docker-compose.yaml up -d

docker-down:
	docker compose -f docker/docker-compose.yaml down
