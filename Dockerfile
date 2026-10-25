# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=0.1.0
ARG COMMIT=dev
ARG BUILD_DATE=unknown

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN LDFLAGS="-s -w \
    -X github.com/pionus/pionus-go/internal/version.Version=${VERSION} \
    -X github.com/pionus/pionus-go/internal/version.Commit=${COMMIT} \
    -X github.com/pionus/pionus-go/internal/version.BuildDate=${BUILD_DATE}" \
    && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
       go build -trimpath -ldflags "$LDFLAGS" -o /out/pionus . \
    && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
       go build -trimpath -ldflags "$LDFLAGS" -o /out/pionus-migrate ./cmd/pionus-migrate

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -u 10001 pionus \
    && mkdir -p /app/data /app/uploads /app/logs \
    && chown -R pionus:pionus /app

ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /out/pionus ./pionus
COPY --from=builder /out/pionus-migrate ./pionus-migrate
COPY --from=builder /src/config.yaml ./config.yaml
COPY --from=builder /src/markdowns ./markdowns
COPY --from=builder /src/migration ./migration

USER pionus
EXPOSE 8087

CMD ["./pionus"]
