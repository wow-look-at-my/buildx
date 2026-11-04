
FROM busybox
copy Dockerfile .

FROM busybox as base
COPY Dockerfile .
		