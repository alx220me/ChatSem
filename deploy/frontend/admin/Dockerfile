FROM node:20-alpine AS builder
WORKDIR /app

COPY frontend/admin/package*.json ./
RUN npm ci --prefer-offline

COPY frontend/admin/ .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist/ /usr/share/nginx/html/admin/
EXPOSE 80
