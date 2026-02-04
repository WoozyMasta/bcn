GO ?= go
LINTER  ?= golangci-lint
ALIGNER ?= betteralign

.PHONY: test verify vet fmt fmt-check lint align align-fix check tidy download tools release-notes

check: fmt-check vet lint align test

fmt:
	gofmt -w .

fmt-check:
	@gofmt -l . | tee /dev/stderr | read; \
	if [ $$? -eq 0 ]; then \
		echo "gofmt: files need formatting"; \
		exit 1; \
	fi

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

verify:
	$(GO) mod verify

tidy:
	$(GO) mod tidy

download:
	$(GO) mod download

lint:
	$(LINTER) run ./...

align:
	$(ALIGNER) ./...

align-fix:
	$(ALIGNER) -apply ./...

tools:
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	$(GO) install github.com/dkorunic/betteralign/cmd/betteralign@latest

release-notes:
	@awk '\
	/^<!--/,/^-->/ { next } \
	/^## \[[0-9]+\.[0-9]+\.[0-9]+\]/ { if (found) exit; found=1; next } found { print } \
	' CHANGELOG.md
