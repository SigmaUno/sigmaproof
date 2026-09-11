.PHONY: check docs-check test build fmt-check vectors-check
check: docs-check fmt-check vectors-check test
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

vectors-check:
	python3 scripts/generate_merkle_vectors.py --check
	python3 scripts/generate_commitment_vectors.py --check
