FROM golang as builder
COPY . /src
WORKDIR /src
RUN go build -o mptcp-proxy

FROM debian
COPY --from=builder /etc/ssl/certs /etc/ssl/certs
COPY --from=builder /src/mptcp-proxy /
ENTRYPOINT ["/mptcp-proxy"]