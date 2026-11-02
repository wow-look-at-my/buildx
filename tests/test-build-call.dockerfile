
FROM busybox AS first
RUN --mount=type=secret,target=/etc/passwd,required=true --mount=type=ssh true

FROM alpine AS second
RUN --mount=type=secret,id=unused --mount=type=ssh,id=ssh2 true

FROM scratch AS third
ARG BAR
RUN --mount=type=secret,id=second${BAR} true

FROM third AS target
COPY --from=first /foo /
RUN --mount=type=ssh,id=ssh3,required true

FROM second
	