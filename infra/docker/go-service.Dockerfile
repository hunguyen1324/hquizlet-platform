# Go service builder – hỗ trợ cả root main.go (gateway/quiz) và cmd/server/main.go (auth/study)
# Dev5 – fix build path sau khi Dev1/Dev2 refactor sang cmd/internal structure

FROM golang:1.25-alpine AS build

ARG SERVICE
WORKDIR /src/services/${SERVICE}

# Include workspace modules such as pkg/cache used by Study.
COPY go.work go.work.sum /src/
COPY services /src/services
COPY pkg /src/pkg

# Download and verify dependencies without changing module manifests.
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
LABEL org.opencontainers.image.source="https://github.com/hunguyen1324/hquizlet-platform"
WORKDIR /app
COPY --from=build /out/service /app/service

EXPOSE 8080 8081 8082 8083 8084 8085
CMD ["/app/service"]
