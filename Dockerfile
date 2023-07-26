FROM bitnami/kubectl:1.20.9 as kubectl
FROM golang:1.20-alpine as builder
WORKDIR /app

COPY go.mod ./
COPY cmd/main.go main.go
RUN go mod download
RUN go build -o acrpurgectl main.go

FROM alpine:latest

# Azure-Cli dependencies
RUN apk update
RUN apk add bash py3-pip gcc musl-dev python3-dev libffi-dev openssl-dev cargo make
RUN pip install --upgrade pip
RUN pip install azure-cli
COPY --from=kubectl /opt/bitnami/kubectl/bin/kubectl /usr/local/bin/

WORKDIR /root/
COPY --from=builder /app/acrpurgectl /root
