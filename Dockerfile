FROM golang:1.23-alpine AS build

WORKDIR /src
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal

ARG APP
RUN case "${APP}" in \
      koffy-billing-api) CMD_DIR=billing-api ;; \
      koffy-gateway) CMD_DIR=ai-gateway ;; \
      *) CMD_DIR="${APP}" ;; \
    esac && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/app ./cmd/${CMD_DIR}

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && adduser -D -H appuser
USER appuser
WORKDIR /app

COPY --from=build /out/app /app/app

EXPOSE 8080 8081
ENTRYPOINT ["/app/app"]
