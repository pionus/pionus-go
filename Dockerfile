# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/pionus .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -u 10001 pionus \
    && mkdir -p /app/data /app/uploads /app/logs \
    && chown -R pionus:pionus /app

ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /out/pionus ./pionus
COPY --from=builder /src/config.yaml ./config.yaml
COPY --from=builder /src/markdowns ./markdowns

USER pionus
EXPOSE 8087

CMD ["./pionus"]
