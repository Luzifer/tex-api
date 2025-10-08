FROM golang:1.25-alpine@sha256:6104e2bbe9f6a07a009159692fe0df1a97b77f5b7409ad804b17d6916c635ae5 AS builder

COPY . /go/src/github.com/Luzifer/tex-api
WORKDIR /go/src/github.com/Luzifer/tex-api

RUN set -ex \
 && apk add --no-cache \
      git \
 && go install \
      -ldflags "-X main.version=$(git describe --tags || git rev-parse --short HEAD || echo dev)" \
      -mod=readonly


FROM alpine:3.22@sha256:56b31e2dadc083b6b067d6cd4e97a9c6e5a953e6595830c60d9197589ff88ad4

LABEL maintainer="Knut Ahlers <knut@ahlers.me>"

ENV SCRIPT=/usr/local/bin/tex-build.sh

RUN set -ex \
 && apk --no-cache add \
      bash \
      ca-certificates \
      texlive \
      texlive-xetex \
      texmf-dist-binextra \
      texmf-dist-fontsrecommended \
      texmf-dist-fontutils \
      texmf-dist-langenglish \
      texmf-dist-langfrench \
      texmf-dist-langgerman \
      texmf-dist-latexextra \
      texmf-dist-pictures \
      texmf-dist-xetex

COPY --from=builder /go/bin/tex-api /usr/local/bin/
COPY                tex-build.sh    /usr/local/bin/

EXPOSE 3000
VOLUME ["/storage"]

ENTRYPOINT ["/usr/local/bin/tex-api"]
CMD ["--"]

# vim: set ft=Dockerfile:
