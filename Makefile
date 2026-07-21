SHELL := /bin/bash

APP_VERSION ?= $(shell cat VERSION 2>/dev/null || echo 0.1.1)
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

.PHONY: build up down ps logs smoke

build:
	APP_VERSION=$(APP_VERSION) GIT_COMMIT=$(GIT_COMMIT) BUILD_DATE=$(BUILD_DATE) docker compose build

up:
	APP_VERSION=$(APP_VERSION) GIT_COMMIT=$(GIT_COMMIT) BUILD_DATE=$(BUILD_DATE) docker compose up -d --wait

down:
	docker compose down --remove-orphans

ps:
	docker compose ps

logs:
	docker compose logs --tail=100 -f

smoke:
	bash scripts/mvp_smoke_test.sh
