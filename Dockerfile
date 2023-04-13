FROM golang as builder
COPY . /src
WORKDIR /src
RUN go build -o server

FROM debian
COPY --from=builder /etc/ssl/certs /etc/ssl/certs
COPY --from=builder /src/server /
CMD ["/server"]