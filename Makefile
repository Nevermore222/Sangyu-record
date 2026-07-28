.PHONY: test vet infra-up infra-down migrate miniapp-test skill-test smoke

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

miniapp-test:
	npm --prefix miniapp test
	npm --prefix miniapp run typecheck

skill-test:
	npm --prefix skills/mock-memoir test

smoke:
	powershell -ExecutionPolicy Bypass -File scripts/vertical-slice.ps1
