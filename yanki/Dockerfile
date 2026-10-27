FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY ortam/ ortam/
COPY yanki/ yanki/
WORKDIR /build/yanki
RUN go build -o /yanki ./cmd/yanki

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /yanki /usr/local/bin/yanki
EXPOSE 9118 9119
CMD ["yanki"]
