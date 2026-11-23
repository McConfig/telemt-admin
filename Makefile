.PHONY: test vet build run
test:
	go test ./...
vet:
	go vet ./...
build:
	CGO_ENABLED=0 go build -trimpath -o tgadmin ./cmd/tgadmin
run:
	go run ./cmd/tgadmin serve
