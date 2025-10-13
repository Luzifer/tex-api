FROM golang:1.25-alpine@sha256:ecb803821dbac0983b03a516adb8407b7dbadfb8771434dc5162a7c471dd27a2 AS builder

COPY . /go/src/github.com/Luzifer/tex-api
WORKDIR /go/src/github.com/Luzifer/tex-api

RUN set -ex \
 && apk add --no-cache \
      git \
 && go install \
      -ldflags "-X main.version=$(git describe --tags || git rev-parse --short HEAD || echo dev)" \
      -mod=readonly


FROM alpine:3.22@sha256:4b7ce07002c69e8f3d704a9c5d6fd3053be500b7f1c69fc0d80990c2ad8dd412

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
