FROM node:20-alpine AS builder
WORKDIR /app

COPY frontend/widget/package*.json ./
RUN npm ci --prefer-offline

COPY frontend/widget/ .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist/ /usr/share/nginx/html/widget/
EXPOSE 80
