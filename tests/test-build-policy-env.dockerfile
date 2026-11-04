FROM busybox:latest AS build
RUN echo stage-build

FROM busybox:latest AS final
RUN echo stage-final
