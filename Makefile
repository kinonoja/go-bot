.PHONY: test lint run-runner migrate-up migrate-down

test:
	cd services/runner && go test ./...
	cd services/api && go test ./...

lint:
	cd services/runner && go vet ./...
	cd services/api && go vet ./...

run-runner:
	cd services/runner && go run ./cmd/runner

migrate-up:
	migrate -path services/api/migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path services/api/migrations -database "$(DATABASE_URL)" down
