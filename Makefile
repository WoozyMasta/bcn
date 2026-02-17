GO          ?= go
LINTER      ?= golangci-lint
ALIGNER     ?= betteralign
BENCHSTAT   ?= benchstat
BENCH_COUNT ?= 6

BENCH_REF_MULTI  ?= bench_baseline_multi_thread.txt
BENCH_REF_SINGLE ?= bench_baseline_single_thread.txt

.PHONY: test bench bench-single-thread bench-fast verify vet check \
	fmt fmt-check lint lint-fix align align-fix \
	tidy download tools tool-golangci-lint tool-betteralign tool-benchstat \
	release-notes

check: verify fmt-check vet lint align test

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
	@tmp=$$(mktemp); \
	BCN_BENCH_LARGE=1 $(GO) test ./... -run=^$$ -bench 'Benchmark' -benchmem -count=$(BENCH_COUNT) | tee "$$tmp"; \
	if [ -f "$(BENCH_REF_MULTI)" ]; then \
		$(BENCHSTAT) "$(BENCH_REF_MULTI)" "$$tmp"; \
	else \
		cp "$$tmp" "$(BENCH_REF_MULTI)" && echo "Baseline multi-thread saved to $(BENCH_REF_MULTI)"; \
	fi; \
	rm -f "$$tmp"

bench-single-thread:
	@tmp=$$(mktemp); \
	GOMAXPROCS=1 $(GO) test ./... -run=^$$ -bench 'Benchmark' -benchmem -count=$(BENCH_COUNT) | tee "$$tmp"; \
	if [ -f "$(BENCH_REF_SINGLE)" ]; then \
		$(BENCHSTAT) "$(BENCH_REF_SINGLE)" "$$tmp"; \
	else \
		cp "$$tmp" "$(BENCH_REF_SINGLE)" && echo "Baseline single-thread saved to $(BENCH_REF_SINGLE)"; \
	fi; \
	rm -f "$$tmp"

bench-fast:
	$(GO) test ./... -run=^$$ -bench 'Benchmark' -benchmem

verify:
	$(GO) mod verify

tidy:
	$(GO) mod tidy

download:
	$(GO) mod download

lint:
	$(LINTER) run ./...

lint-fix:
	$(LINTER) run --fix ./...

align:
	$(ALIGNER) ./...

align-fix:
	$(ALIGNER) -apply ./...

tools: tool-golangci-lint tool-betteralign tool-benchstat

tool-golangci-lint:
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

tool-betteralign:
	$(GO) install github.com/dkorunic/betteralign/cmd/betteralign@latest

tool-benchstat:
	$(GO) install golang.org/x/perf/cmd/benchstat@latest

release-notes:
	@awk '\
	/^<!--/,/^-->/ { next } \
	/^## \[[0-9]+\.[0-9]+\.[0-9]+\]/ { if (found) exit; found=1; next } \
	found { \
		if (/^## \[/) { exit } \
		if (/^$$/) { flush(); print; next } \
		if (/^\* / || /^- /) { flush(); buf=$$0; next } \
		if (/^###/ || /^\[/) { flush(); print; next } \
		sub(/^[ \t]+/, ""); sub(/[ \t]+$$/, ""); \
		if (buf != "") { buf = buf " " $$0 } else { buf = $$0 } \
		next \
	} \
	function flush() { if (buf != "") { print buf; buf = "" } } \
	END { flush() } \
	' CHANGELOG.md
