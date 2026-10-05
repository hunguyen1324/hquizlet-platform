FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm install -g pnpm && pnpm install
COPY . .
ARG VITE_GATEWAY_URL=/api
RUN cd apps/web && VITE_GATEWAY_URL="$VITE_GATEWAY_URL" pnpm run build

FROM nginx:1.27-alpine
LABEL org.opencontainers.image.source="https://github.com/hunguyen1324/hquizlet-platform"
COPY --from=builder /app/apps/web/dist /usr/share/nginx/html
