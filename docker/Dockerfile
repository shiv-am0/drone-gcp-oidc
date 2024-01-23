FROM scratch

RUN apk --no-cache add ca-certificates

ADD release/linux/amd64/drone-oidc /bin/

ENTRYPOINT ["/bin/drone-oidc"]