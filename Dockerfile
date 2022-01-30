FROM uber/go-torch:latest

COPY ./ /workspace/
WORKDIR /workspace
ENV PATH="/go/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/flamegraph:/workspace"
RUN ls -al /workspace
ENTRYPOINT ["/bin/sh", "-c", "/workspace/web_linux"]


