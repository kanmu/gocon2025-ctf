FROM golang:1.25 AS builder
WORKDIR /src
COPY go.mod go.sum /src/
RUN go mod download && go mod verify
COPY main.go /src/
COPY assets /src/assets
RUN go build -o gocon2025-ctf

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=builder /src/gocon2025-ctf /app/gocon2025-ctf
CMD ["./gocon2025-ctf"]
