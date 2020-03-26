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

test: proxy
	go test ./...
