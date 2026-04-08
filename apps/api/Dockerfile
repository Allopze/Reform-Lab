# --- Build stage ---
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/server ./cmd/server

# --- Runtime stage ---
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
		apt-get install -y --no-install-recommends \
			ca-certificates \
			tzdata \
			poppler-utils \
			libreoffice-core \
			libreoffice-writer \
			ffmpeg \
		&& rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /bin/server /app/server
COPY migrations/ /app/migrations/

EXPOSE 8080
ENTRYPOINT ["/app/server"]
