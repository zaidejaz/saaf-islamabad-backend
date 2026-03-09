.PHONY: run build swagger db-up db-down test clean

run: swagger
	go run main.go

build: swagger
	go build -o bin/server main.go

swagger:
	swag init --parseDependency --parseInternal

db-up:
	docker compose up -d

db-down:
	docker compose down

db-reset:
	docker compose down -v && docker compose up -d

test:
	go test ./... -v

clean:
	rm -rf bin/ docs/
