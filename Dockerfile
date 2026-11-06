# Build Stage for Gateway Go Binary
FROM golang:1.26-alpine AS builder
WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/gateflow ./cmd/gateway

# Production Minimal Stage
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /home/appuser

COPY --from=builder /app/gateflow /home/appuser/gateflow
COPY migrations /home/appuser/migrations

RUN chown -R appuser:appgroup /home/appuser
USER appuser

EXPOSE 8080 8081

ENTRYPOINT ["/home/appuser/gateflow"]
