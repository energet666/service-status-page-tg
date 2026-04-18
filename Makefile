SHELL := /bin/sh

APP_NAME := service-status-page
BIN_DIR := bin
BIN := $(BIN_DIR)/$(APP_NAME)
GO_CACHE := /tmp/service-status-page-go-build
GO_MOD_CACHE := /tmp/service-status-page-go-mod

.DEFAULT_GOAL := help

.PHONY: help install web-build build run test clean

help:
	@printf '%s\n' 'Available targets:'
	@printf '  %-12s %s\n' 'install' 'Install frontend dependencies'
	@printf '  %-12s %s\n' 'web-build' 'Build the Svelte frontend into web/dist'
	@printf '  %-12s %s\n' 'build' 'Build frontend and Go binary'
	@printf '  %-12s %s\n' 'run' 'Build frontend, then run the Go server'
	@printf '  %-12s %s\n' 'test' 'Run Go tests with local caches under /tmp'
	@printf '  %-12s %s\n' 'clean' 'Remove generated build output'

install:
	cd web && npm install

web-build:
	cd web && npm run build

build: web-build
	mkdir -p $(BIN_DIR)
	env GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o $(BIN) ./cmd/server

run: web-build
	env GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go run ./cmd/server

test:
	env GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go test ./...

clean:
	rm -rf $(BIN_DIR) web/dist
