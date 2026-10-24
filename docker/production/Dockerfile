# syntax=docker/dockerfile:1.7

# --- Stage 1: Build ---
FROM golang:1.26.7 AS builder

WORKDIR /app

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GOPRIVATE=github.com/whagency/* \
    GONOSUMDB=github.com/whagency/* \
    GONOPROXY=github.com/whagency/*

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=secret,id=github_token \
    sh -eu -c ' \
      github_token="$(cat /run/secrets/github_token)"; \
      git config --global url."https://${github_token}@github.com/".insteadOf "https://github.com/"; \
      go mod download; \
      git config --global --unset-all url."https://${github_token}@github.com/".insteadOf \
    '

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=secret,id=github_token \
    sh -eu -c ' \
      github_token="$(cat /run/secrets/github_token)"; \
      git config --global url."https://${github_token}@github.com/".insteadOf "https://github.com/"; \
      go build -ldflags="-w -s -X main.version=$(git log --format="%h" -1)" -o ./notification-service ./cmd/notification-service; \
      go build -ldflags="-w -s -X main.version=$(git log --format="%h" -1)" -o ./migrate ./cmd/commands/migrate; \
      git config --global --unset-all url."https://${github_token}@github.com/".insteadOf \
    '

# --- Stage 2: Runtime ---
FROM gcr.io/distroless/static-debian12 AS runner

WORKDIR /app

COPY --from=builder /app/notification-service /usr/local/bin/notification-service
COPY --from=builder /app/migrate /usr/local/bin/migrate
COPY --from=builder /app/data/ ./data/
COPY --from=builder /usr/local/go/lib/time/zoneinfo.zip /zoneinfo.zip

ENV ZONEINFO=/zoneinfo.zip

USER nonroot:nonroot

CMD ["notification-service"]



