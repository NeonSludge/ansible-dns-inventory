.DEFAULT_GOAL := build

build:
	go build -ldflags '-s -w' -o ./dns-inventory ./cmd/dns-inventory 

