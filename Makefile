$(VERBOSE).SILENT:
.DEFAULT_GOAL := _default
CUR_DIR := $(PWD)
export DOCKER_BUILDKIT=1
platform := $(shell uname -p)
checkarm := arm
EXECUTABLE=dns-inventory
VERSION=$(shell git describe --tags --always)

Color_Off:=\033[0m
# Regular Colors
Black:=\033[0;30m
Red:=\033[0;31m
Green:=\033[0;32m
Yellow:=\033[0;33m
Blue:=\033[0;34m
Purple:=\033[0;35m
Cyan:=\033[0;36m
White:=\033[0;37m


define greeting
	@clear
	@echo "$(Yellow)"
    @echo "    ___               _ __    __           ____                      __                  "
    @echo "   /   |  ____  _____(_) /_  / /__        /  _/___ _   _____  ____  / /_____  _______  __"
    @echo "  / /| | / __ \/ ___/ / __ \/ / _ \______ / // __ \ | / / _ \/ __ \/ __/ __ \/ ___/ / / /"
    @echo " / ___ |/ / / (__  ) / /_/ / /  __/_____// // / / / |/ /  __/ / / / /_/ /_/ / /  / /_/ / "
    @echo "/_/  |_/_/ /_/____/_/_.___/_/\___/     /___/_/ /_/|___/\___/_/ /_/\__/\____/_/   \__, /  "
    @echo "                                                                                /____/   "
	@echo "$(Color_Off)"
endef

define cleanup
	@COMPOSE_PROFILES=dns,etcd docker compose -f docker/docker-compose.yml down
	@rm -rf ./$(EXECUTABLE)_$(VERSION)_*
endef

_init: 
	$(call greeting)

build-windows: ## Make Windows binary
	env GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -X 'github.com/NeonSludge/ansible-dns-inventory/internal/build.Version=$(VERSION)' -X 'github.com/NeonSludge/ansible-dns-inventory/internal/build.Time=$(shell date -u +%Y%m%dT%H%M%SZ)'" -o ./$(EXECUTABLE)_$(VERSION)_amd64_windows.exe ./cmd/$(EXECUTABLE)

build-darwin: ## Make Darwin binary (ARM/AMD64)
	env GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -X 'github.com/NeonSludge/ansible-dns-inventory/internal/build.Version=$(VERSION)' -X 'github.com/NeonSludge/ansible-dns-inventory/internal/build.Time=$(shell date -u +%Y%m%dT%H%M%SZ)'" -o ./$(EXECUTABLE)_$(VERSION)_amd64_darwin ./cmd/$(EXECUTABLE)
	env GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w -X 'github.com/NeonSludge/ansible-dns-inventory/internal/build.Version=$(VERSION)' -X 'github.com/NeonSludge/ansible-dns-inventory/internal/build.Time=$(shell date -u +%Y%m%dT%H%M%SZ)'" -o ./$(EXECUTABLE)_$(VERSION)_arm64_darwin ./cmd/$(EXECUTABLE)

build-linux: ## Make Linux binary (ARM/AMD64)
	env GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X 'github.com/NeonSludge/ansible-dns-inventory/internal/build.Version=$(VERSION)' -X 'github.com/NeonSludge/ansible-dns-inventory/internal/build.Time=$(shell date -u +%Y%m%dT%H%M%SZ)'" -o ./$(EXECUTABLE)_$(VERSION)_amd64_linux ./cmd/$(EXECUTABLE)
	env GOOS=linux GOARCH=arm64 go build -ldflags "-s -w -X 'github.com/NeonSludge/ansible-dns-inventory/internal/build.Version=$(VERSION)' -X 'github.com/NeonSludge/ansible-dns-inventory/internal/build.Time=$(shell date -u +%Y%m%dT%H%M%SZ)'" -o ./$(EXECUTABLE)_$(VERSION)_arm64_linux ./cmd/$(EXECUTABLE)

build: build-linux build-darwin build-windows ## Make all binaries

test-dns: _init build ## Run DNS tests
	@echo " "
	@COMPOSE_PROFILES=dns docker compose -f docker/docker-compose.yml up -d
	@sleep 5
	@chmod +x ./$(EXECUTABLE)_$(VERSION)_*
	@echo " "
	@echo "------------- RUN TESTS------------------"
	@docker compose -f docker/docker-compose.yml exec -it multitool-dns /bin/bash -c "/app/$(EXECUTABLE)_$(VERSION)_$(platform)64_linux -tree"
	$(call cleanup)

test-etcd: _init build ## Run etcd tests
	@echo " "
	@COMPOSE_PROFILES=etcd docker compose -f docker/docker-compose.yml up -d
	@sleep 5
	@chmod +x ./$(EXECUTABLE)_$(VERSION)_*
	@chmod +x docker/config/etcd/init.sh
	@bash docker/config/etcd/init.sh
	@echo " "
	@echo "------------- RUN TESTS------------------"
	@docker compose -f docker/docker-compose.yml exec -it multitool-etcd /bin/bash -c "/app/$(EXECUTABLE)_$(VERSION)_$(platform)64_linux -tree"
	$(call cleanup)

test: test-dns test-etcd ## Run all tests

image: build ## Build image
	@echo " "

help:
	$(call greeting)
	grep -E '(^[a-z].*[^:]\s*##)|(^##)' $(MAKEFILE_LIST) | \
		perl -pe "s/Makefile://" | perl -pe "s/^##\s*//" | \
		awk ' \
			BEGIN { FS = ":.*##" } \
			$$2 { printf "\033[32m%-30s\033[0m %s\n", $$1, $$2 } \
			!$$2 { printf " \033[33m%-30s\033[0m\n", $$1 } \
		'

## make exec="command": ## Выполнить команду в контейнере
_default:
	if [ '$(exec)' ]; then \
		COMPOSE_PROFILES=tools docker compose -f docker/docker-compose.yml up -d; \
		docker compose exec -it -f docker/docker-compose.yml multitool -v "$(CUR_DIR):/app" exec -it -c '$(exec)'; \
	else \
		make help; \
	fi