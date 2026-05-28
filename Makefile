.PHONY: build run dev deps

deps:
	go mod tidy
	cd web && npm install

build: deps
	cd web && npm run build
	go build -o mac-monitor ./cmd/mac-monitor

run: build
	./mac-monitor

# Start Go server + Vite dev server side-by-side
dev:
	go run ./cmd/mac-monitor &
	cd web && npm run dev
