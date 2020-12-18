FROM golang:alpine as builder

RUN apk update && apk add make git

COPY . $GOPATH/src/github.com/AlbinoDrought/creamy-inbound-stuff
WORKDIR $GOPATH/src/github.com/AlbinoDrought/creamy-inbound-stuff

RUN CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=amd64 \
  make install

FROM scratch

COPY --from=builder /go/bin/creamy-inbound-stuff /go/bin/creamy-inbound-stuff
ENTRYPOINT ["/go/bin/creamy-inbound-stuff"]
