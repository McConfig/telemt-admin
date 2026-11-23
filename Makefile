.PHONY: test vet build frontend run
test:
	go test ./...
vet:
	go vet ./...
frontend:
	cd frontend && npm ci && npm run build
build: frontend
	CGO_ENABLED=0 go build -trimpath -o tgadmin ./cmd/tgadmin
run:
	cd frontend && npm run build
	go run ./cmd/tgadmin serve
