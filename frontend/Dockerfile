FROM node:20-alpine AS builder

WORKDIR /app

# Копируем package files
COPY package*.json ./
RUN npm ci

# Копируем исходники
COPY . .

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
