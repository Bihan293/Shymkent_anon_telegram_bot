FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY bot/go.mod bot/go.sum ./
RUN go mod download

COPY bot/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /server /server

CMD ["/server"]
