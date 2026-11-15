# syntax=docker/dockerfile:1
# Go service builder – hỗ trợ cả root main.go (gateway/quiz) và cmd/server/main.go (auth/study)
# Dev5 – fix build path sau khi Dev1/Dev2 refactor sang cmd/internal structure

FROM golang:1.25-alpine AS build

ARG SERVICE
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}
WORKDIR /src/services/${SERVICE}

# Include workspace modules such as pkg/cache used by Study.
COPY go.work go.work.sum /src/
COPY services /src/services
COPY pkg /src/pkg
COPY packages /src/packages

# Download and verify dependencies without changing module manifests.
RUN --mount=type=cache,id=hquizlet-gomod,target=/go/pkg/mod \
    attempts=0; \
    until go mod download; do \
      attempts=$((attempts + 1)); \
      if [ "$attempts" -ge 4 ]; then echo "Dependency download failed after 4 attempts" >&2; exit 1; fi; \
      echo "Dependency download interrupted; retrying ($attempts/4)..." >&2; \
      sleep $((attempts * 5)); \
    done; \
    go mod verify

# Build: ưu tiên ./cmd/server nếu tồn tại (auth, study sau refactor),
# fallback về root package (gateway, quiz vẫn dùng root main.go)
RUN --mount=type=cache,id=hquizlet-gomod,target=/go/pkg/mod \
    --mount=type=cache,id=hquizlet-gobuild,target=/root/.cache/go-build \
    if [ -d "./cmd/server" ]; then \
      go build -o /out/service ./cmd/server; \
    else \
      go build -o /out/service .; \
    fi

RUN --mount=type=cache,id=hquizlet-gomod,target=/go/pkg/mod \
    --mount=type=cache,id=hquizlet-gobuild,target=/root/.cache/go-build \
    if [ "$SERVICE" = "class" ]; then go build -o /out/shard-migrate ./cmd/shard-migrate && go build -o /out/caption-tool ./cmd/caption-tool; fi

FROM alpine:3.20
ARG SERVICE
ARG YTDLP_VERSION=2026.10.26
RUN if [ "$SERVICE" = "class" ]; then \
      apk add --no-cache ffmpeg python3 py3-pip ca-certificates && \
      python3 -m venv /opt/caption-tools && \
      /opt/caption-tools/bin/pip install --no-cache-dir "yt-dlp==${YTDLP_VERSION}"; \
    fi
ENV YOUTUBE_YTDLP_BIN=/opt/caption-tools/bin/yt-dlp
LABEL org.opencontainers.image.source="https://github.com/hunguyen1324/hquizlet-platform"
WORKDIR /app
COPY --from=build /out/ /app/

EXPOSE 8080 8081 8082 8083 8084 8085
CMD ["/app/service"]
