
FROM widnyana/go-builder:1.22-alpine

WORKDIR /app

RUN apk update \ 
    && apk add --no-cache \
    ca-certificates \
    curl \
    htop \
    nano \
    tzdata \
    && update-ca-certificates

RUN go install github.com/cosmtrek/air@latest

RUN pwd

COPY . .

RUN go mod tidy


