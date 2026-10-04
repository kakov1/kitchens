FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /build/kitchen ./cmd/kitchens \
    && go clean -cache -modcache

FROM alpine:latest

WORKDIR /root/
COPY --from=builder /build/kitchen .

EXPOSE 8000
CMD ["./kitchen"]