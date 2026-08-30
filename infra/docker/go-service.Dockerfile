# Go service builder – hỗ trợ cả root main.go (gateway/quiz) và cmd/server/main.go (auth/study)
# Dev5 – fix build path sau khi Dev1/Dev2 refactor sang cmd/internal structure

FROM golang:1.23-alpine AS build

ARG SERVICE
WORKDIR /src/services/${SERVICE}

# Copy chỉ service cần build – mỗi service có go.mod riêng, không cần workspace
COPY services/${SERVICE} .

RUN go mod download
RUN go mod verify

# Build: ưu tiên ./cmd/server nếu tồn tại (auth, study sau refactor),
# fallback về root package (gateway, quiz vẫn dùng root main.go)
RUN if [ -d "./cmd/server" ]; then \
      go build -o /out/service ./cmd/server; \
    else \
      go build -o /out/service .; \
    fi

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/service /app/service

EXPOSE 8080 8081 8082 8083 8084 8085
CMD ["/app/service"]
