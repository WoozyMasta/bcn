GO          ?= go
LINTER      ?= golangci-lint
ALIGNER     ?= betteralign
BENCHSTAT   ?= benchstat
CC          ?= cc
BENCH_COUNT ?= 6
FUZZTIME    ?= 20s
ASMGEN_REF  ?= ./internal/simd/asmgen

BCDEC_REV    := 93628fe5627102fe5187b7eeb99122dec6612c36
BCDEC_URL    := https://raw.githubusercontent.com/iOrange/bcdec/$(BCDEC_REV)/bcdec.h
BCDEC_SHA256 := f54dcae4a2f5dc3008f66814fb57653134a568cdce461c1b4bb3dfc7d6061204
BCDEC_HEADER := testdata/gen/bcdec.h

BENCH_REF_MULTI  ?= bench_baseline_multi.txt
BENCH_REF_SINGLE ?= bench_baseline_single.txt

.PHONY: check ci

check: generate verify tidy fmt vet lint-fix align-fix test test-race test-pure test-race-pure fuzz
ci: download tools-ci generate-check verify tidy-check fmt-check vet lint align test test-pure fuzz

.PHONY: generate generate-check

generate:
	GOWORK=off $(GO) -C $(ASMGEN_REF) run . \
		-out ../kernels_amd64.s -stubs ../kernels_stubs_amd64.go -pkg simd
	gofmt -w internal/simd/kernels_stubs_amd64.go

generate-check: generate
	git diff --exit-code -- internal/simd

.PHONY: gen-parity-fixtures gen-parity-fixtures-clean gen-parity-fixtures-check

gen-parity-fixtures:
	mkdir -p testdata/parity
	curl -sSfLo $(BCDEC_HEADER) $(BCDEC_URL)
	echo "$(BCDEC_SHA256)  $(BCDEC_HEADER)" | sha256sum --check --status -
	$(CC) -std=c99 -O2 -Wall -Wextra -Werror -o testdata/gen/gen_parity testdata/gen/gen_parity.c
	./testdata/gen/gen_parity

gen-parity-fixtures-clean:
	rm -f $(BCDEC_HEADER) testdata/gen/gen_parity testdata/gen/gen_parity.exe

gen-parity-fixtures-check: gen-parity-fixtures
	git diff --exit-code -- testdata/parity

.PHONY: test test-race test-pure test-race-pure fuzz

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

test-pure:
	$(GO) test -tags purego ./...

test-race-pure:
	$(GO) test -tags purego -race ./...

fuzz:
	@for t in FuzzScoreBC1Palette FuzzPackBC1Indices FuzzAlphaKernels FuzzDecodeNoPanic FuzzDecodeBC6HNoPanic; do \
		echo "> $$t"; \
		$(GO) test -run '^$$' -fuzz "^$$t$$" -fuzztime $(FUZZTIME) . || exit 1; \
	done

.PHONY: bench bench-multi bench-single \
	bench-fast bench-multi-fast bench-single-fast \
	bench-reset bench-multi-reset bench-single-reset

bench: bench-multi bench-single

bench-multi:
	@tmp=$$(mktemp); \
	BCN_BENCH_LARGE=1 $(GO) test ./... -run=^$$ -bench 'Benchmark' -benchmem -count=$(BENCH_COUNT) | tee "$$tmp"; \
	if [ -f "$(BENCH_REF_MULTI)" ]; then \
		$(BENCHSTAT) "$(BENCH_REF_MULTI)" "$$tmp"; \
	else \
		cp "$$tmp" "$(BENCH_REF_MULTI)" && echo "Baseline multi-thread saved to $(BENCH_REF_MULTI)"; \
	fi; \
	rm -f "$$tmp"

bench-single:
	@tmp=$$(mktemp); \
	GOMAXPROCS=1 $(GO) test ./... -run=^$$ -bench 'Benchmark' -benchmem -count=$(BENCH_COUNT) | tee "$$tmp"; \
	if [ -f "$(BENCH_REF_SINGLE)" ]; then \
		$(BENCHSTAT) "$(BENCH_REF_SINGLE)" "$$tmp"; \
	else \
		cp "$$tmp" "$(BENCH_REF_SINGLE)" && echo "Baseline single-thread saved to $(BENCH_REF_SINGLE)"; \
	fi; \
	rm -f "$$tmp"

bench-fast: bench-multi-fast bench-single-fast

bench-multi-fast:
	BCN_BENCH_LARGE=1 $(GO) test ./... -run=^$$ -bench 'Benchmark' -benchmem

bench-single-fast:
	GOMAXPROCS=1 $(GO) test ./... -run=^$$ -bench 'Benchmark' -benchmem

bench-reset: bench-multi-reset bench-single-reset

bench-multi-reset:
	rm -f "$(BENCH_REF_MULTI)"

bench-single-reset:
	rm -f "$(BENCH_REF_SINGLE)"

.PHONY: download verify vet tidy tidy-check fmt fmt-check lint lint-fix align align-fix

download:
	$(GO) mod download
	GOWORK=off $(GO) -C $(ASMGEN_REF) mod download

verify:
	$(GO) mod verify
	GOWORK=off $(GO) -C $(ASMGEN_REF) mod verify

vet:
	$(GO) vet ./...
	GOWORK=off $(GO) -C $(ASMGEN_REF) vet ./...

tidy:
	$(GO) mod tidy
	GOWORK=off $(GO) -C $(ASMGEN_REF) mod tidy

tidy-check:
	@$(GO) mod tidy
	@GOWORK=off $(GO) -C $(ASMGEN_REF) mod tidy
	@git diff --stat --exit-code -- go.mod go.sum internal/simd/asmgen/go.mod internal/simd/asmgen/go.sum || ( \
		echo "go mod tidy: repository is not tidy"; \
		exit 1; \
	)

fmt:
	gofmt -w .

fmt-check:
	@files="$$(gofmt -l .)"; \
	if [ -n "$$files" ]; then \
		echo "$$files"; \
		echo "gofmt: files need formatting"; \
		exit 1; \
	fi

lint:
	$(LINTER) run ./...

lint-fix:
	$(LINTER) run --fix ./...

align:
	$(ALIGNER) ./...

align-fix:
	-$(ALIGNER) -apply ./...
	$(ALIGNER) ./...

.PHONY: tools tools-ci tool-golangci-lint tool-betteralign tool-benchstat

tools: tool-golangci-lint tool-betteralign tool-benchstat
tools-ci: tool-golangci-lint tool-betteralign

tool-golangci-lint:
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

tool-betteralign:
	$(GO) install github.com/dkorunic/betteralign/cmd/betteralign@latest

tool-benchstat:
	$(GO) install golang.org/x/perf/cmd/benchstat@latest

.PHONY: release-notes

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
