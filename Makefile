include .env
export

run:
	go run ./cmd/api

db-up:
	docker compose up -d db

db-down:
	docker compose down

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

migrate-new:
	migrate create -ext sql -dir migrations -seq $(name)