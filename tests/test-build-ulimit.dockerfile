
FROM busybox AS build
RUN ulimit -n > first > /ulimit
FROM scratch
COPY --from=build /ulimit /
	