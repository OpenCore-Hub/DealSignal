.PHONY: build test lint security trivy-fs trivy-image run clean compose-up compose-down loadtest loadtest-public-link loadtest-signed-url loadtest-search loadtest-assistant

build:
	go build -o bin/server ./cmd/server

test:
	go test -race ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

lint:
	golangci-lint run ./...

security:
	@which govulncheck > /dev/null 2>&1 || go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

# Trivy filesystem scan of the whole repository. Requires Docker.
# Pinned scanner image to keep scans reproducible.
TRIVY_VERSION ?= 0.62.0
PROJECT_ROOT := $(abspath ../..)
TRIVY := docker run --rm -v $(PROJECT_ROOT):/app -w /app aquasec/trivy:$(TRIVY_VERSION)

trivy-fs:
	$(TRIVY) filesystem --scanners vuln --severity HIGH,CRITICAL --exit-code 1 --ignore-unfixed /app

trivy-image: build
	docker build -t dealsignal-api:latest -f Dockerfile .
	mkdir -p tmp
	docker save dealsignal-api:latest -o tmp/dealsignal-api.tar
	$(TRIVY) image --scanners vuln --severity HIGH,CRITICAL --exit-code 1 --ignore-unfixed --input /app/apps/api/tmp/dealsignal-api.tar || (rm -f tmp/dealsignal-api.tar; exit 1)
	rm -f tmp/dealsignal-api.tar

run:
	go run ./cmd/server

clean:
	rm -rf bin/

compose-up:
	docker-compose -f docker-compose.yml up --build -d

compose-down:
	docker-compose -f docker-compose.yml down -v

loadtest:
	@echo "Usage: make loadtest-public-link TARGET=http://host:port LINK_TOKEN=xxx"
	@echo "       make loadtest-signed-url TARGET=http://host:port WORKSPACE_SLUG=xxx DOCUMENT_ID=xxx API_TOKEN=xxx"
	@echo "       make loadtest-search TARGET=http://host:port WORKSPACE_SLUG=xxx API_TOKEN=xxx QUERY=xxx"
	@echo "       make loadtest-assistant TARGET=http://host:port WORKSPACE_SLUG=xxx API_TOKEN=xxx"

loadtest-public-link:
	@which k6 > /dev/null 2>&1 || (echo "k6 not installed"; exit 1)
	k6 run scripts/loadtest/public-link.js -e TARGET=$(TARGET) -e LINK_TOKEN=$(LINK_TOKEN)

loadtest-signed-url:
	@which k6 > /dev/null 2>&1 || (echo "k6 not installed"; exit 1)
	k6 run scripts/loadtest/signed-url.js -e TARGET=$(TARGET) -e WORKSPACE_SLUG=$(WORKSPACE_SLUG) -e DOCUMENT_ID=$(DOCUMENT_ID) -e API_TOKEN=$(API_TOKEN)

loadtest-search:
	@which k6 > /dev/null 2>&1 || (echo "k6 not installed"; exit 1)
	k6 run scripts/loadtest/search.js -e TARGET=$(TARGET) -e WORKSPACE_SLUG=$(WORKSPACE_SLUG) -e API_TOKEN=$(API_TOKEN) -e QUERY=$(QUERY)

loadtest-assistant:
	@which k6 > /dev/null 2>&1 || (echo "k6 not installed"; exit 1)
	k6 run scripts/loadtest/assistant-chat.js -e TARGET=$(TARGET) -e WORKSPACE_SLUG=$(WORKSPACE_SLUG) -e API_TOKEN=$(API_TOKEN)

