# use container signing with (https://github.com/sigstore/cosign)

# build stage
FROM golang:1.17-alpine3.15 AS build-env
RUN apk add build-base

WORKDIR /src

COPY go.mod ./
COPY go.sum ./
COPY *.go ./
RUN go mod download

# run swagger then build swag doc
RUN go get github.com/swaggo/swag/cmd/swag
# run swagger from GOPATH
RUN swag init -g main.go

RUN go build -o alyagofn


# final stage
FROM alpine:3.14

WORKDIR /app
# copy swagger doc
COPY --from=build-env /src/docs /app/docs
# copy binary
COPY --from=build-env /src/alyagofn /app/


EXPOSE 9090
# run binary
ENTRYPOINT ./alyagofn
