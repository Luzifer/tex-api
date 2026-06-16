FROM golang:1.26.4-alpine@sha256:7a3e50096189ad57c9f9f865e7e4aa8585ed1585248513dc5cda498e2f41812c AS builder

COPY . /go/src/github.com/Luzifer/tex-api
WORKDIR /go/src/github.com/Luzifer/tex-api

RUN set -ex \
 && apk add --no-cache \
      git \
 && go install \
      -ldflags "-X main.version=$(git describe --tags || git rev-parse --short HEAD || echo dev)" \
      -mod=readonly


FROM alpine:3.24@sha256:f5064d3e5f88c467c714509f491853ab2d951932c5cad699c0cb969dcec6f3b4

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
