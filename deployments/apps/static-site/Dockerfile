FROM docker.io/library/golang@sha256:d9b6a1beb9474adc2a1ce842573639b44b2a0dd49b6669f51d3821baf9ec37c9 AS runtime
COPY main.go /src/main.go
RUN CGO_ENABLED=0 go build -o /dev-site /src/main.go

FROM docker.io/library/node@sha256:54f84b4cf8432b21aaffd5d3ce8d41e12cea0d10d33c09d2f3c9879d49d6b446
COPY --from=runtime /dev-site /usr/local/bin/dev-site
USER node
ENTRYPOINT ["/usr/local/bin/dev-site"]
