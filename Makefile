.DEFAULT_GOAL := build

EXECUTABLE=dns-inventory
VERSION=$(shell git describe --tags --always)

build-windows:
	CGO_ENABLED=0 env GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -X 'github.com/NeonSludge/ansible-dns-inventory/internal/build.Version=$(VERSION)' -X 'github.com/NeonSludge/ansible-dns-inventory/internal/build.Time=$(shell date -u +%Y%m%dT%H%M%SZ)'" -o ./$(EXECUTABLE)_$(VERSION)_amd64_windows.exe ./cmd/$(EXECUTABLE)

build-darwin:
	CGO_ENABLED=0 env GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -X 'github.com/NeonSludge/ansible-dns-inventory/internal/build.Version=$(VERSION)' -X 'github.com/NeonSludge/ansible-dns-inventory/internal/build.Time=$(shell date -u +%Y%m%dT%H%M%SZ)'" -o ./$(EXECUTABLE)_$(VERSION)_amd64_darwin ./cmd/$(EXECUTABLE)
	CGO_ENABLED=0 env GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w -X 'github.com/NeonSludge/ansible-dns-inventory/internal/build.Version=$(VERSION)' -X 'github.com/NeonSludge/ansible-dns-inventory/internal/build.Time=$(shell date -u +%Y%m%dT%H%M%SZ)'" -o ./$(EXECUTABLE)_$(VERSION)_arm64_darwin ./cmd/$(EXECUTABLE)

build-linux:
	CGO_ENABLED=0 env GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X 'github.com/NeonSludge/ansible-dns-inventory/internal/build.Version=$(VERSION)' -X 'github.com/NeonSludge/ansible-dns-inventory/internal/build.Time=$(shell date -u +%Y%m%dT%H%M%SZ)'" -o ./$(EXECUTABLE)_$(VERSION)_amd64_linux ./cmd/$(EXECUTABLE)
	CGO_ENABLED=0 env GOOS=linux GOARCH=arm64 go build -ldflags "-s -w -X 'github.com/NeonSludge/ansible-dns-inventory/internal/build.Version=$(VERSION)' -X 'github.com/NeonSludge/ansible-dns-inventory/internal/build.Time=$(shell date -u +%Y%m%dT%H%M%SZ)'" -o ./$(EXECUTABLE)_$(VERSION)_arm64_linux ./cmd/$(EXECUTABLE)

build: build-linux build-darwin build-windows
