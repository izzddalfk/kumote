FROM golang:1.24-alpine3.20 AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download -x

# Copy source code
COPY ./cmd/assistant ./cmd/assistant
COPY ./internal/assistant ./internal/assistant
COPY ./internal/shared ./internal/shared

# Build
WORKDIR /app/cmd/assistant
RUN go build -o assistant

FROM alpine:3.20
RUN apk add ca-certificates tzdata

# Copy binary
COPY --from=builder /app/cmd/assistant/assistant .

# Run
CMD ["./assistant"]