ARG BASE_IMAGE_PREFIX=

FROM ${BASE_IMAGE_PREFIX}node:22-alpine AS builder

WORKDIR /app

ENV CI=true

COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY . .

ARG VITE_API_URL=http://localhost:8080
ARG VITE_WS_URL=ws://localhost:8080
ARG VITE_API_KEY=secret-api-key-12345

ENV VITE_API_URL=$VITE_API_URL \
    VITE_WS_URL=$VITE_WS_URL \
    VITE_API_KEY=$VITE_API_KEY

RUN npm run build

FROM ${BASE_IMAGE_PREFIX}nginx:alpine

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
