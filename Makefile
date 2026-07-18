SHELL := /bin/bash

.PHONY: build up down ps logs smoke

build:
	docker compose build

up:
	docker compose up -d --wait

down:
	docker compose down --remove-orphans

ps:
	docker compose ps

logs:
	docker compose logs --tail=100 -f

smoke:
	bash scripts/mvp_smoke_test.sh
