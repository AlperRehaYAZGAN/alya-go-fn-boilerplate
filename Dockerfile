# use container signing with (https://github.com/sigstore/cosign)

# build stage
FROM golang:1.17-alpine3.15 AS build-env
RUN apk add build-base

# run swagger then build swag doc
RUN go get -u github.com/go-swagger/go-swagger/cmd/swagger

RUN swag init -g main.go 

ADD . /src

RUN cd /src && go build -o goapp

# final stage
FROM alpine
WORKDIR /app
# copy swagger doc
COPY --from=build-env /src/docs /app/docs
# copy binary
COPY --from=build-env /src/goapp /app/

# run binary
ENTRYPOINT ./goapp