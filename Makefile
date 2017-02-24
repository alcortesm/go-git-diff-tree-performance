.PHONY: clean

bin = go-git-diff-tree-performance
arch = amd64
os = linux

tag = $(bin)
img = $(tag).img

$(bin): *.go
	GOOS=$(os) GOARCH=$(arch) CGO_ENABLED=0 go build -o $(bin)

docker: $(bin)
	docker build --tag $(tag) .
	docker save -o $(img) $(tag)

clean:
	- rm -f $(bin)
	- rm -f $(img)
