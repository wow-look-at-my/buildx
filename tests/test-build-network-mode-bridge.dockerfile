
FROM busybox AS build
RUN ip a show eth0 | awk '/inet / {split($2, a, "/"); print a[1]}' > /ip-bridge.txt
RUN --network=host ip a show eth0 | awk '/inet / {split($2, a, "/"); print a[1]}' > /ip-host.txt
FROM scratch
COPY --from=build /ip*.txt /