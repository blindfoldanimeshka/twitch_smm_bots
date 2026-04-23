FROM golang:1.26-alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -o bot logic.go

# Финальный образ
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata ffmpeg streamlink

WORKDIR /root/

# Копируем бинарник из builder
COPY --from=builder /app/bot .

# Открываем порт
EXPOSE 8080

# Запускаем приложение
CMD ["./bot"]
