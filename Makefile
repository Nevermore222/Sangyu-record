.PHONY: test vet infra-up infra-down migrate

test:
	go test ./...

vet:
	go vet ./...

infra-up:
	docker compose -f deploy/local/compose.yaml up -d postgres redis minio minio-init

infra-down:
	docker compose -f deploy/local/compose.yaml down

migrate:
	go run github.com/pressly/goose/v3/cmd/goose@v3.27.3 -dir migrations up
