FROM golang:alpine as builder

ADD . /go/src/github.com/Luzifer/tex-api
WORKDIR /go/src/github.com/Luzifer/tex-api

RUN set -ex \
 && apk add --update git \
 && go install -ldflags "-X main.version=$(git describe --tags || git rev-parse --short HEAD || echo dev)"

FROM alpine:latest

LABEL maintainer "Knut Ahlers <knut@ahlers.me>"

RUN set -ex \
 && apk --no-cache add \
      bash \
      ca-certificates \
      texlive-full

COPY --from=builder /go/bin/tex-api /usr/local/bin/
COPY --from=builder /go/src/github.com/Luzifer/tex-api/tex-build.sh /usr/local/bin/

EXPOSE 3000
VOLUME ["/storage"]

ENTRYPOINT ["/usr/local/bin/tex-api"]
CMD ["--script", "/usr/local/bin/tex-build.sh"]

# vim: set ft=Dockerfile:
