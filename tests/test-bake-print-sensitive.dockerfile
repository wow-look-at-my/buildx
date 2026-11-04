
FROM busybox
ARG HELLO
RUN echo "Hello ${HELLO}"
	