.PHONY: default install test
all: default install test

gosec:
	go get github.com/securego/gosec/cmd/gosec

sec:
	@gosec ./...
	@echo "[OK] Go security check was completed!"

proxy:
	export GOPROXY=https://goproxy.cn

init:
	go get github.com/jteeuwen/go-bindata/...

res:
	go-bindata -pkg "httplive" -o "bindata.go" public/...

default: proxy
	go fmt ./...&&revive -exclude bindata.go .&&goimports -w .&&golangci-lint run --skip-files=bindata.go --enable-all

# go get -u github.com/gobuffalo/packr/v2/packr2
install: proxy
	packr2
	go install -ldflags="-s -w" ./...
	upx ~/go/bin/httplive

# https://hub.docker.com/_/golang
# docker run --rm -v "$PWD":/usr/src/myapp -v "$HOME/dockergo":/go -w /usr/src/myapp golang make docker
# docker run --rm -it -v "$PWD":/usr/src/myapp -w /usr/src/myapp golang bash
# 静态连接 glibc
docker:
	go install -v -x -a -ldflags '-extldflags "-static" -s -w' ./...
test: proxy
	go test ./...
