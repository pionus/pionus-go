FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o pionus .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /build/pionus .
COPY --from=builder /build/config.yaml .
COPY --from=builder /build/theme ./theme
COPY --from=builder /build/markdowns ./markdowns

EXPOSE 8087
CMD ["./pionus"]
