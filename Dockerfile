FROM golang:latest as builder
RUN go install github.com/qsocket/qs-netcat@latest
COPY ./qsocket /usr/bin/qsocket
ENTRYPOINT ["qsocket"]