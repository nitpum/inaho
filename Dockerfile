FROM golang:1.17-alpine AS builder

WORKDIR /build
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY *.go .
RUN go build -o inaho-yamato

FROM alpine:3.10
WORKDIR /app
COPY --from=builder /build/inaho-yamato /bin/inaho-yamato
ENTRYPOINT ["/bin/inaho-yamato"]
