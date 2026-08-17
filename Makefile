.DEFAULT_GOAL := help

.PHONY: help test fmt fmt-check vet check pre-commit secrets

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "%-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

test: ## Run all tests
	go test ./...

fmt: ## Format Go source files
	gofmt -w $$(find . -name '*.go' -not -path './.direnv/*')

fmt-check: ## Verify that Go source files are formatted
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './.direnv/*'))"

vet: ## Run go vet
	go vet ./...

check: fmt-check vet test ## Run all checks

pre-commit: ## Run all pre-commit hooks against staged changes
	pre-commit run

secrets: ## Scan the complete Git history for secrets
	gitleaks git --redact --verbose .
