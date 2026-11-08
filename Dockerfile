# Multi-stage build. BASE_IMAGE_PREFIX is optional (Docker Hub mirror), e.g.
#   docker compose build --build-arg BASE_IMAGE_PREFIX=mirror.yandex.ru/mirrors/docker/library/
ARG BASE_IMAGE_PREFIX=

FROM ${BASE_IMAGE_PREFIX}golang:1.26-alpine AS builder

WORKDIR /src

ENV CGO_ENABLED=0 \
    GOOS=linux

COPY go.mod go.sum ./
RUN go mod download

COPY logic.go ./
COPY internal ./internal

RUN go build -trimpath -ldflags="-s -w" -o /out/bot .

FROM ${BASE_IMAGE_PREFIX}alpine:3.22

ENV TZ=Europe/Moscow

# ffmpeg + streamlink are needed only for optional stream audio transcription,
# but they are small enough to keep the runtime image self-contained.
RUN apk add --no-cache ca-certificates tzdata ffmpeg streamlink

WORKDIR /app

COPY --from=builder /out/bot ./bot

EXPOSE 8080

ENTRYPOINT ["./bot"]
