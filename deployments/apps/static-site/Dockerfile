ARG SYNC_IMAGE=registry.k8s.io/git-sync/git-sync@sha256:088e7365787d859427135eff39fd60e1c6480d194443f4678d41a96c7255465f
FROM ${SYNC_IMAGE} AS sync
FROM docker.io/library/golang@sha256:d9b6a1beb9474adc2a1ce842573639b44b2a0dd49b6669f51d3821baf9ec37c9 AS hook
COPY main.go /src/main.go
RUN CGO_ENABLED=0 go build -o /build-site /src/main.go

FROM docker.io/library/node@sha256:54f84b4cf8432b21aaffd5d3ce8d41e12cea0d10d33c09d2f3c9879d49d6b446
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=sync /git-sync /git-sync
COPY --from=hook /build-site /usr/local/bin/build-site
ENTRYPOINT ["/git-sync"]
