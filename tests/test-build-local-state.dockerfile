
FROM busybox:latest AS base
COPY foo /etc/foo
RUN cp /etc/foo /etc/bar

FROM scratch
COPY --from=base /etc/bar /bar
