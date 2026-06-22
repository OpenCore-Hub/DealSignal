.PHONY: build test lint run clean compose-up compose-down

build:
	go build -o bin/server ./cmd/server

test:
	go test -race -cover ./...

lint:
	golangci-lint run ./...

run:
	go run ./cmd/server

clean:
	rm -rf bin/

compose-up:
	docker-compose -f docker-compose.yml up --build -d

compose-down:
	docker-compose -f docker-compose.yml down -v
