.PHONY: default test
all: default test

proxy:
	export GOPROXY=https://goproxy.cn

init:
	go get github.com/jteeuwen/go-bindata/...

res:
	go-bindata -pkg "httplive" -o "bindata.go" public/...

default: proxy
	go fmt ./...&&revive -exclude bindata.go .&&goimports -w .&&golangci-lint run --skip-files=bindata.go --enable-all&& go install -ldflags="-s -w" ./...

install: proxy
	go install -ldflags="-s -w" ./...

test: proxy
	go test ./...
