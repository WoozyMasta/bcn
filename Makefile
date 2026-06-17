GO          ?= go
LINTER      ?= golangci-lint
ALIGNER     ?= betteralign
BENCHSTAT   ?= benchstat
BENCH_COUNT ?= 6
BENCH_REF   ?= bench_baseline.txt
FUZZTIME    ?= 20s
ASMGEN_REF  ?= ./internal/simd/asmgen

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
	@for t in FuzzScoreDXT1Palette FuzzPackDXT1Indices FuzzAlphaKernels FuzzDecodeNoPanic; do \
		echo "> $$t"; \
		$(GO) test -run '^$$' -fuzz "^$$t$$" -fuzztime $(FUZZTIME) . || exit 1; \
	done

.PHONY: bench bench-fast bench-reset

bench:
	@tmp=$$(mktemp); \
	$(GO) test ./... -run=^$$ -bench 'Benchmark' -benchmem -count=$(BENCH_COUNT) | tee "$$tmp"; \
	if [ -f "$(BENCH_REF)" ]; then \
		$(BENCHSTAT) "$(BENCH_REF)" "$$tmp"; \
	else \
		cp "$$tmp" "$(BENCH_REF)" && echo "Baseline saved to $(BENCH_REF)"; \
	fi; \
	rm -f "$$tmp"

bench-fast:
	$(GO) test ./... -run=^$$ -bench 'Benchmark' -benchmem

bench-reset:
	rm -f "$(BENCH_REF)"

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
