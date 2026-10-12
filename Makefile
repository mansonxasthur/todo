.PHONY: migrate-create obs-up obs-down obs-logs

# Explicit -f disables compose's automatic override file, so list all of them.
OBS_COMPOSE = docker compose \
	-f docker-compose.yml \
	-f docker-compose.override.yml \
	-f docker-compose.observability.yml

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
