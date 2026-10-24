# Makefile for notification-service

GO_RUN := GOTOOLCHAIN=go1.25.8+auto go run
GOVULNCHECK := $(GO_RUN) golang.org/x/vuln/cmd/govulncheck@v1.1.4
GOLANGCI_LINT := $(GO_RUN) github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4
SWAG := $(GO_RUN) github.com/swaggo/swag/cmd/swag@v1.16.6

# Check for vulnerable dependencies using govulncheck.
.PHONY: vulnerable-check
vulnerable-check:
	$(GOVULNCHECK) ./...

# Linting using golangci-lint.
.PHONY: lint
lint:
	$(GOLANGCI_LINT) run --config .golangci.yml

# Swagger code generation.
.PHONY: swagger-gen
swagger-gen:
	$(SWAG) init -g ./cmd/notification-service/app/http_server.go -o docs --parseInternal --parseDependency

# Generate local service artifacts.
.PHONY: generate
generate: swagger-gen
