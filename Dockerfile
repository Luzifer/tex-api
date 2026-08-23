FROM golang:1.27.0-alpine@sha256:4c9fe60190a2a3350ddc51de80d0224b8a6698d12bdfc999fee45ea9d6c46dbc AS builder

COPY . /go/src/github.com/Luzifer/tex-api
WORKDIR /go/src/github.com/Luzifer/tex-api

RUN set -ex \
 && apk add --no-cache \
      git \
 && go install \
      -ldflags "-X main.version=$(git describe --tags || git rev-parse --short HEAD || echo dev)" \
      -mod=readonly


FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

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
