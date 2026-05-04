FROM golang:alpine AS builder

# CGO required by mattn/go-sqlite3
RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o /app/im-server .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/im-server .
COPY --from=builder /app/web ./web
COPY --from=builder /app/config ./config

RUN mkdir -p /app/uploads /app/data

EXPOSE 8888 8080

ENV IM_SERVER_IP=0.0.0.0
ENV IM_WEB_ADDR=:8080
ENV IM_DB_PATH=/app/data/im.db
ENV IM_UPLOAD_DIR=/app/uploads

CMD ["./im-server", "-env", "prod"]
