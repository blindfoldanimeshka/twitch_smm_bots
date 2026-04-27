# Используем базовый образ Ubuntu для сборки
FROM ubuntu:22.04 AS builder

# Устанавливаем Go
RUN apt-get update && apt-get install -y wget && \
    wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz && \
    rm go1.23.0.linux-amd64.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /app

# Копируем go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -o bot logic.go

# Финальный образ
FROM ubuntu:22.04

# Устанавливаем переменные окружения для неинтерактивной установки
ENV DEBIAN_FRONTEND=noninteractive
ENV TZ=Europe/Moscow

# Устанавливаем зависимости
RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    ffmpeg \
    python3 \
    python3-pip \
    && pip3 install --no-cache-dir streamlink \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /root/

# Копируем бинарник из builder
COPY --from=builder /app/bot .

# Открываем порт
EXPOSE 8080

# Запускаем приложение
CMD ["./bot"]
