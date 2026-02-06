GO ?= go
LINTER  ?= golangci-lint
ALIGNER ?= betteralign

.PHONY: test bench bench-baseline bench-compare verify vet fmt fmt-check lint align align-fix check tidy download tools release-notes

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

bench:
	$(GO) test -test.fullpath=true -run=^$$ -bench '^BenchmarkEncodeBlock(DXT1|DXT5)$$' -benchmem
	BCN_BENCH_LARGE=1 $(GO) test -test.fullpath=true -run=^$$ -bench '^BenchmarkEncodeImage(DXT1|DXT5)$$' -benchmem

bench-baseline:
	GOMAXPROCS=1 BCN_BENCH_LARGE=1 $(GO) test -run=^$$ -bench . -benchmem -count=6 2>&1 | tee bench-baseline-1.txt
	BCN_BENCH_LARGE=1 $(GO) test -run=^$$ -bench . -benchmem -count=6 2>&1 | tee bench-baseline.txt

bench-compare:
	GOMAXPROCS=1 BCN_BENCH_LARGE=1 $(GO) test -run=^$$ -bench . -benchmem -count=6 2>&1 | tee bench-new-1.txt
	benchstat bench-baseline-1.txt bench-new-1.txt
	BCN_BENCH_LARGE=1 $(GO) test -run=^$$ -bench . -benchmem -count=6 2>&1 | tee bench-new.txt
	benchstat bench-baseline.txt bench-new.txt

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
	$(GO) install golang.org/x/perf/cmd/benchstat@latest

release-notes:
	@awk '\
	/^<!--/,/^-->/ { next } \
	/^## \[[0-9]+\.[0-9]+\.[0-9]+\]/ { if (found) exit; found=1; next } found { print } \
	' CHANGELOG.md
