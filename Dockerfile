FROM golang:latest

ADD . /go/src/github.com/cj123/quotedb

WORKDIR /go/src/github.com/cj123/quotedb

RUN go get .
RUN go build .

EXPOSE 8990

ENTRYPOINT /go/src/github.com/cj123/quotedb/quotedb
