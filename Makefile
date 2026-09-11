.PHONY: check docs-check test build fmt-check
check: docs-check fmt-check test
	go vet ./...

docs-check:
	python3 scripts/check_repository.py

fmt-check:
	@test -z "$$(gofmt -l cmd internal pkg)" || (gofmt -l cmd internal pkg; exit 1)

test:
	go test -race ./...

build:
	mkdir -p bin
	go build -trimpath -o bin/sigmaproof ./cmd/sigmaproof
	go build -trimpath -o bin/sigmaproofd ./cmd/sigmaproofd
