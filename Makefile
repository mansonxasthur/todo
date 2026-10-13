.PHONY: check test vet fmt-check up down logs migrate-create obs-up obs-down obs-logs

.DEFAULT_GOAL := check

# Explicit -f disables compose's automatic override file, so list all of them.
OBS_COMPOSE = docker compose \
	-f docker-compose.yml \
	-f docker-compose.override.yml \
	-f docker-compose.observability.yml

# Everything to run before committing. Stops at the first failing step.
check: fmt-check vet test

# The postgres tests use testcontainers, so Docker must be running.
test:
	go test -race ./...

vet:
	go vet ./...

# gofmt -l exits 0 even when it lists files, so fail on any output.
fmt-check:
	@test -z "$$(gofmt -l .)" || { echo "unformatted:"; gofmt -l .; exit 1; }

# Dev stack with live reload (compose picks up docker-compose.override.yml).
up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f app

migrate-create:
ifndef  name
	$(error usage: make migrate-create name=<migration_name>)
endif
	migrate create -ext sql -dir migrations -format 20060102150405 $(name)

# Start the app plus the LGTM stack. Grafana: http://localhost:3000
obs-up:
	$(OBS_COMPOSE) up -d --build

# Stop everything. Add VOLUMES=1 to also delete stored telemetry data.
obs-down:
	$(OBS_COMPOSE) down $(if $(VOLUMES),-v)

obs-logs:
	$(OBS_COMPOSE) logs -f otel-collector app
