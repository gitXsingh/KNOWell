FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/knowell-api ./cmd/api

FROM gcr.io/distroless/base-debian12

WORKDIR /app

COPY --from=builder /bin/knowell-api /app/knowell-api
COPY --from=builder /app/migrations /app/migrations

COPY --from=builder /usr/bin/wget /usr/bin/wget

EXPOSE 8080

ENV APP_ADDR=:8080
ENV MIGRATIONS_DIR=/app/migrations

USER 65532

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["/app/knowell-api"]
