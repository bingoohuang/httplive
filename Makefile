.PHONY: default install test
all: default install test

APPNAME=httplive

gosec:
	go get github.com/securego/gosec/cmd/gosec

sec:
	@gosec ./...
	@echo "[OK] Go security check was completed!"

proxy:
	export GOPROXY=https://goproxy.cn

init:
	go get github.com/jteeuwen/go-bindata/...

default: proxy
	go fmt ./...&&revive -exclude bindata.go .&&goimports -w .&&golangci-lint run --skip-files=bindata.go --enable-all

# go get github.com/markbates/pkger/cmd/pkger
install: proxy
	pkger
	go install -ldflags="-s -w" ./...
	ls -lh ~/go/bin/$(APPNAME)
	upx ~/go/bin/$(APPNAME)
	ls -lh ~/go/bin/$(APPNAME)
package: install
	mv ~/go/bin/$(APPNAME) ~/go/bin/$(APPNAME)-v1.0.1-darwin-amd64
	gzip ~/go/bin/$(APPNAME)-v1.0.1-darwin-amd64
	ls -lh ~/go/bin/$(APPNAME)*

# https://hub.docker.com/_/golang
# docker run --rm -v "$PWD":/usr/src/myapp -v "$HOME/dockergo":/go -w /usr/src/myapp golang make docker
# docker run --rm -it -v "$PWD":/usr/src/myapp -w /usr/src/myapp golang bash
# 静态连接 glibc
docker:
	docker run --rm -v "$$PWD":/usr/src/myapp -v "$$HOME/dockergo":/go -w /usr/src/myapp golang make dockerinstall
	ls -lh ~/dockergo/bin/$(APPNAME)
	upx ~/dockergo/bin/$(APPNAME)
	ls -lh ~/dockergo/bin/$(APPNAME)
	mv ~/dockergo/bin/$(APPNAME)  ~/dockergo/bin/$(APPNAME)-v1.0.1-amd64-glibc2.28
	gzip ~/dockergo/bin/$(APPNAME)-v1.0.1-amd64-glibc2.28
	ls -lh ~/dockergo/bin/$(APPNAME)*

dockerinstall:
	go install -v -x -a -ldflags '-extldflags "-static" -s -w' ./...

test: proxy
	go test ./...
