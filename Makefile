APP_NAME := cinema-booking
MAIN := ./cmd/main.go

.PHONY: build run test lint fmt vet tidy docker-up docker-down clean

build:
	go build -o bin/$(APP_NAME) $(MAIN)

run:
	go run $(MAIN)

test:
	go test ./... -v

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy

docker-up:
	docker compose up -d

docker-down:
	docker compose down

clean:
	rm -rf bin/
