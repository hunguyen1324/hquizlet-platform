FROM node:22-alpine

WORKDIR /app

COPY apps/web/package*.json ./
RUN npm install

COPY apps/web ./

EXPOSE 5173
CMD ["sh", "-c", "test -d node_modules/vite || npm install; npm run dev -- --host 0.0.0.0"]
