# ---------- build stage ----------
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Зависимости
COPY go.mod go.sum ./
RUN go mod download

# Исходники
COPY . .

# Сборка
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o alice-telegram .

# ---------- runtime stage ----------
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates

# Бинарник
COPY --from=builder /app/alice-telegram /app/alice-telegram

EXPOSE 8080

ENTRYPOINT ["/app/alice-telegram"]
