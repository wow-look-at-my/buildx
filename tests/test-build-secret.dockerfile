
FROM busybox AS build
RUN --mount=type=secret,id=token cat /run/secrets/token | tee /token
FROM scratch
COPY --from=build /token /
	