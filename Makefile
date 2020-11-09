.DEFAULT_GOAL := build

EXECUTABLE=dns-inventory
VERSION=$(shell git describe --tags --always)

build-windows:
	env GOOS=windows GOARCH=amd64 go build -ldflags '-s -w' -o ./$(EXECUTABLE)_$(VERSION)_amd64_windows.exe ./cmd/$(EXECUTABLE)

build-darwin:
	env GOOS=darwin GOARCH=amd64 go build -ldflags '-s -w' -o ./$(EXECUTABLE)_$(VERSION)_amd64_darwin ./cmd/$(EXECUTABLE)

build-linux:
	env GOOS=linux GOARCH=amd64 go build -ldflags '-s -w' -o ./$(EXECUTABLE)_$(VERSION)_amd64_linux ./cmd/$(EXECUTABLE)

build: build-linux build-darwin build-windows

