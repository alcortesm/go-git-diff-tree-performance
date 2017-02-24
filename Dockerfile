FROM alpine:latest
RUN apk add --no-cache git
ADD go-git-diff-tree-performance /bin/
ENTRYPOINT ["/bin/go-git-diff-tree-performance"]
