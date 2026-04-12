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
ENV HOME=/tmp/reform-home

RUN apt-get update && \
		apt-get install -y --no-install-recommends \
			ca-certificates \
			tzdata \
			ghostscript \
			poppler-utils \
			librsvg2-bin \
			libreoffice-core \
			libreoffice-calc \
			libreoffice-impress \
			libreoffice-writer \
			ffmpeg \
			libheif-examples \
			tesseract-ocr \
			tesseract-ocr-spa \
		&& rm -rf /var/lib/apt/lists/* \
		&& groupadd --system app \
		&& useradd --system --gid app --create-home --home-dir /home/app app \
		&& mkdir -p /data /tmp/reform-home /app \
		&& chown -R app:app /app /data /tmp/reform-home /home/app
WORKDIR /app
COPY --from=builder /bin/server /app/server
COPY migrations/ /app/migrations/
COPY scripts/docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod 755 /app/docker-entrypoint.sh

EXPOSE 8080
ENTRYPOINT ["/app/docker-entrypoint.sh", "/app/server"]
