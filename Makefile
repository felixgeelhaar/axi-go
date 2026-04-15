.PHONY: build test vet fmt fmt-check lint cover security check install-hooks

GO       := go
LINT     := golangci-lint
NOX      := nox
COVERCTL := coverctl
COVERAGE := coverage.out

build:
	$(GO) build ./...

test:
	$(GO) test ./... -count=1

vet:
	$(GO) vet ./...

fmt:
	$(LINT) fmt ./...

fmt-check:
	@test -z "$$($(LINT) fmt --diff ./...)" || (echo "Files need formatting. Run 'make fmt' to fix." && $(LINT) fmt --diff ./... && exit 1)

lint:
	$(LINT) run ./...

cover:
	$(GO) test ./... -coverprofile=$(COVERAGE) -count=1
	$(GO) tool cover -func=$(COVERAGE)
	@$(COVERCTL) record --profile $(COVERAGE) 2>/dev/null || true

security:
	$(NOX) scan .

check: fmt lint test security
	@echo "All checks passed."

install-hooks:
	@echo "Installing git hooks..."
	@mkdir -p .git/hooks
	@cp scripts/pre-commit .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "Done. Pre-commit hook installed."
