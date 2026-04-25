FROM node:20-alpine AS builder

WORKDIR /app

# Копируем package files
COPY package*.json ./
RUN npm ci

# Копируем исходники
COPY . .

# Build arguments для Vite
ARG VITE_API_URL=http://localhost:8080
ARG VITE_WS_URL=ws://localhost:8080
ARG VITE_API_KEY=secret-api-key-12345

# Устанавливаем переменные окружения для build
ENV VITE_API_URL=$VITE_API_URL
ENV VITE_WS_URL=$VITE_WS_URL
ENV VITE_API_KEY=$VITE_API_KEY

# Собираем production build
RUN npm run build

# Production образ с nginx
FROM nginx:alpine

# Копируем собранные файлы
COPY --from=builder /app/dist /usr/share/nginx/html

# Копируем nginx конфиг
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
