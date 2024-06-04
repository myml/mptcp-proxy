FROM golang as builder
COPY . /src
WORKDIR /src
RUN go build -o mptcp-proxy
RUN go build ./cmd/client
RUN go build ./cmd/server

FROM debian
COPY --from=builder /etc/ssl/certs /etc/ssl/certs
COPY --from=builder /src/mptcp-proxy /
COPY --from=builder /src/client /
COPY --from=builder /src/server /
ENTRYPOINT ["/mptcp-proxy"]