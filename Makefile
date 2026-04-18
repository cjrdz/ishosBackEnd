.PHONY: help dev test coverage-focus test-integration migrate seed bootstrap bootstrap-admin build docker-up docker-down clean generate lint format bruno backup-pb deploy

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

dev: ## Start development server
	go run ./cmd/api

test: ## Run all tests
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

coverage-focus: ## Run coverage only for currently unit-tested internal packages
	go test -race -covermode=atomic -coverprofile=coverage.out ./internal/apperror ./internal/events ./internal/services ./internal/validator
	go tool cover -func=coverage.out | tail -n 1

test-integration: ## Run integration tests only
	go test -v ./tests/integration/...

migrate: ## Run database migrations
	go run ./cmd/migrate

seed: ## Seed database with test data
	bash ./scripts/seed_all.sh

bootstrap: ## Create first admin user
	bash ./scripts/bootstrap_admin.sh

bootstrap-admin: ## Create first admin user (supports .bootstrap_admin.env or interactive prompts)
	bash ./scripts/bootstrap_admin.sh

build: ## Build production binary
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build -o bin/server ./cmd/api

docker-up: ## Start Docker Compose
	docker compose up --build

docker-down: ## Stop Docker Compose
	docker compose down -v

clean: ## Clean build artifacts and test data
	rm -rf bin/ coverage.out coverage.html
	rm -f pb_data/*.db

generate: ## Generate Ent code
	go generate ./ent

lint: ## Run linters
	golangci-lint run

format: ## Format code
	go fmt ./...
	goimports -w .

bruno: ## Run Bruno API tests
	cd ishosfactorytest && bruno run .

backup-pb: ## Backup PocketBase data
	./scripts/backup_pb.sh

deploy: ## Deploy to production
	./scripts/deploy.sh
