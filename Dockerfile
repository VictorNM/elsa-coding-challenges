# Build stage
FROM golang:1.22.5 AS builder
ENV GO111MODULE=on
ARG CACHE_DIR=/tmp

# Working directory
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN --mount=type=cache,id=cache-go,target=$CACHE_DIR/.cache CGO_ENABLED=0 GOOS=linux go build -o server cmd/main.go

# Run stage
FROM alpine:3.18

WORKDIR /app
COPY --from=builder /app/server .
COPY config /app/config

EXPOSE 8080 8081
# Run server command
ENTRYPOINT ["/app/server"]