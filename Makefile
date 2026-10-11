.PHONY: migrate-create

migrate-create:
ifndef  name
	$(error usage: make migrate-create name=<migration_name>)
endif
	migrate create -ext sql -dir migrations -format 20060102150405 $(name)