FROM golang:1.26-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /external-dns-huaweicloud ./cmd/webhook

FROM alpine:3.21

RUN apk --no-cache add ca-certificates
COPY --from=builder /external-dns-huaweicloud /bin/external-dns-huaweicloud

USER nobody:nobody
ENTRYPOINT ["/bin/external-dns-huaweicloud"]
