FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY ortam/ ortam/
COPY baskala/ baskala/
WORKDIR /build/baskala
RUN go build -o /baskala ./cmd/baskala

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /baskala /usr/local/bin/baskala
EXPOSE 9122 9123
CMD ["baskala"]
