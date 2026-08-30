FROM golang:1.23-alpine AS build

ARG SERVICE
WORKDIR /src

COPY services/${SERVICE} ./services/${SERVICE}
WORKDIR /src/services/${SERVICE}
RUN go mod tidy

RUN go build -o /out/service .

FROM alpine:3.20

ARG SERVICE
WORKDIR /app
COPY --from=build /out/service /app/service

EXPOSE 8080 8081 8082 8083 8084 8085
CMD ["/app/service"]
