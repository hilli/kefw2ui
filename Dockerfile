# Stage 1: Build frontend
FROM oven/bun:1 AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/bun.lockb* ./
RUN bun install --frozen-lockfile
COPY frontend/ ./
RUN bun run build

# Stage 2: Build Go binary
FROM golang:1.25-alpine AS backend
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/build ./frontend/build
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o kefw2ui .

# Stage 3: Runtime
FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -u 1000 kefw2ui
RUN mkdir -p /home/kefw2ui/.config/kefw2 /home/kefw2ui/.cache/kefw2 /data/tailscale && chown -R kefw2ui:kefw2ui /home/kefw2ui /data/tailscale
USER kefw2ui
WORKDIR /home/kefw2ui
COPY --from=backend /app/kefw2ui /usr/local/bin/
EXPOSE 8080
ENTRYPOINT ["kefw2ui"]
CMD ["--bind", "0.0.0.0", "--port", "8080"]
