FROM golang:1.24-trixie AS builder
RUN mkdir /workdir
COPY . /workdir
RUN cd /workdir && make

FROM gcr.io/distroless/base-nossl-debian12
COPY --from=builder /workdir/docker-telegram-bot /bin/docker-telegram-bot
ENTRYPOINT ["/bin/docker-telegram-bot"]
