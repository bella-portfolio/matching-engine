# ---- Build Stage ----
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o matching-engine .

# ---- Runtime Stage ----
FROM alpine:3.20

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/matching-engine /usr/local/bin/matching-engine

EXPOSE 8080

ENTRYPOINT ["matching-engine"]
