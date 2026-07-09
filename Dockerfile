# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /easy-share .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /easy-share /app/easy-share
RUN mkdir -p /data
ENV DATA_DIR=/data PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/easy-share"]
