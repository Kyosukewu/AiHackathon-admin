.PHONY: build run generate-static

build:
	go build -o bin/server cmd/server/main.go

run: build
	./bin/server

generate-static:
	go run cmd/generate-static/main.go 