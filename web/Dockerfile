FROM node:20-alpine AS builder
WORKDIR /app/web

COPY web/package*.json ./
RUN npm ci

COPY web/ .
RUN npm run build

FROM nginx:1.27-alpine
COPY --from=builder /app/web/dist /usr/share/nginx/html
COPY docker/nginx.prod.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
