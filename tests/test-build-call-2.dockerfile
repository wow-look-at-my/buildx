
# build defines stage for compiling the binary
FROM alpine AS build
RUN true

FROM busybox as second
RUN false

FROM alpine
RUN false

# binary returns the compiled binary
FROM second AS binary
	