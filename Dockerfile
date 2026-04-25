# syntax=docker/dockerfile:1

FROM node:24-alpine AS web-build
WORKDIR /src/web

COPY web/package*.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS go-build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY --from=web-build /src/web/dist ./web/dist

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/service-status-page ./cmd/server

FROM alpine:3.22
WORKDIR /app

RUN apk add --no-cache ca-certificates \
	&& addgroup -S app \
	&& adduser -S -G app app \
	&& mkdir -p /app/data \
	&& chown -R app:app /app

COPY --from=go-build /out/service-status-page /app/service-status-page
COPY --from=web-build /src/web/dist /app/web/dist
COPY checks.json /app/checks.json

ENV HTTP_ADDR=:8080
ENV DATA_FILE=/app/data/state.json
ENV CHECKS_FILE=/app/checks.json

EXPOSE 8080

USER app
ENTRYPOINT ["/app/service-status-page"]
