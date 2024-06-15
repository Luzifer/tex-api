FROM golang:alpine as builder

COPY . /go/src/github.com/Luzifer/tex-api
WORKDIR /go/src/github.com/Luzifer/tex-api

RUN set -ex \
 && apk add --no-cache \
      git \
 && go install \
      -ldflags "-X main.version=$(git describe --tags || git rev-parse --short HEAD || echo dev)" \
      -mod=readonly


FROM luzifer/archlinux:latest

LABEL maintainer "Knut Ahlers <knut@ahlers.me>"

ENV SCRIPT=/usr/local/bin/tex-build.sh

RUN set -ex \
 && pacman --noconfirm -Sy \
      ca-certificates \
      texlive

COPY --from=builder /go/bin/tex-api /usr/local/bin/
COPY                tex-build.sh    /usr/local/bin/

EXPOSE 3000
VOLUME ["/storage"]

ENTRYPOINT ["/usr/local/bin/tex-api"]
CMD ["--"]

# vim: set ft=Dockerfile:
