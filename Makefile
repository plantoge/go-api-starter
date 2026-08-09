.PHONY: run build build-cli test test-integration migrate-platform migrate-tenant swagger

run:
	air

build:
	go build -o bin/api ./cmd/api

build-cli:
	go build -o bin/cli ./cmd/cli

test:
	go test ./... -short

test-integration:
	REQUIRE_TEST_DB=1 go test ./... -v

migrate-platform:
	go run ./cmd/cli migrate platform up

migrate-tenant:
	go run ./cmd/cli migrate tenant up --all

swagger:
	swag init -g cmd/api/main.go -o docs/swagger
