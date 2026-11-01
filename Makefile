HOST_PKGS := ./internal/domain/... ./internal/app/... ./internal/adapter/jsonl/... ./cmd/summary/...

.PHONY: check fmt vet test lint image

check: fmt vet test lint image

fmt:
	@out=$$(gofmt -l cmd internal); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

vet:
	go vet $(HOST_PKGS)

test:
	go test -race -count=1 $(HOST_PKGS)

lint:
	golangci-lint run $(HOST_PKGS)

image:
	docker build -q -t p09-loadgen .
