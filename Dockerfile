FROM golang

MAINTAINER Knut Ahlers <knut@ahlers.me>

ADD . /go/src/github.com/Luzifer/tex-api
WORKDIR /go/src/github.com/Luzifer/tex-api

RUN set -ex \
 && apt-get update \
 && apt-get install -y git ca-certificates \
 && go install -ldflags "-X main.version=$(git describe --tags || git rev-parse --short HEAD || echo dev)" \
 && apt-get install -y texlive-full \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

EXPOSE 3000

VOLUME ["/store"]

ENTRYPOINT ["/go/bin/tex-api"]
CMD ["--"]
