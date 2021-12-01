.PHONY: default test install
all: default test install

app=$(notdir $(shell pwd))
goVersion := $(shell go version)
# echo ${goVersion#go version }
# strip prefix "go version " from output "go version go1.16.7 darwin/amd64"
goVersion2 := $(subst go version ,,$(goVersion))
buildTime := $(shell date '+%Y-%m-%d %H:%M:%S')
gitCommit := $(shell git rev-list -1 HEAD)
app := $(notdir $(shell pwd))
goVersion := $(shell go version)
# echo ${goVersion#go version }
# strip prefix "go version " from output "go version go1.16.7 darwin/amd64"
goVersion2 := $(subst go version ,,$(goVersion))
buildTime := $(shell date '+%Y-%m-%d %H:%M:%S')
gitCommit := $(shell git rev-list -1 HEAD)
# https://stackoverflow.com/a/47510909
pkg := github.com/bingoohuang/httplive
#static := -static
# https://ms2008.github.io/2018/10/08/golang-build-version/
flags = "-extldflags $(static) -s -w -X '$(pkg).buildTime=$(buildTime)' -X $(pkg).gitCommit=$(gitCommit) -X '$(pkg).goVersion=$(goVersion2)'"

tool:
	go get github.com/securego/gosec/cmd/gosec

sec:
	@gosec ./...
	@echo "[OK] Go security check was completed!"

init:
	export GOPROXY=https://goproxy.cn

lint-all:
	golangci-lint run --enable-all

lint:
	golangci-lint run ./...

fmt:
	gofumpt -w .
	gofmt -s -w .
	go mod tidy
	go fmt ./...
	revive .
	goimports -w .
	gci -w -local github.com/daixiang0/gci

install: init
	go install -trimpath -ldflags=${flags}  ./...
	upx ~/go/bin/httplive

linux: init
	GOOS=linux GOARCH=amd64 go install -trimpath -ldflags=${flags}  ./...
	upx ~/go/bin/linux_amd64/httplive
arm: init
	GOOS=linux GOARCH=arm64 go install -trimpath -ldflags=${flags}  ./...
	upx ~/go/bin/linux_arm64/httplive
windows:
	GOOS=windows GOARCH=amd64 go install -ldflags="-s -w" ./...
	upx ~/go/bin/windows_amd64/httplive.exe

upx:
	ls -lh ~/go/bin/${app}
	upx ~/go/bin/${app}
	ls -lh ~/go/bin/${app}
	ls -lh ~/go/bin/linux_amd64/${app}
	upx ~/go/bin/linux_amd64/${app}
	ls -lh ~/go/bin/linux_amd64/${app}

test: init
	#go test -v ./...
	go test -v -race ./...

bench: init
	#go test -bench . ./...
	go test -tags bench -benchmem -bench . ./...

clean:
	rm coverage.out

cover:
	go test -v -race -coverpkg=./... -coverprofile=coverage.out ./...

coverview:
	go tool cover -html=coverage.out

# https://hub.docker.com/_/golang
# docker run --rm -v "$PWD":/usr/src/myapp -v "$HOME/dockergo":/go -w /usr/src/myapp golang make docker
# docker run --rm -it -v "$PWD":/usr/src/myapp -w /usr/src/myapp golang bash
# 静态连接 glibc
docker:
	mkdir -p ~/dockergo
	docker run --rm -v "$$PWD":/usr/src/myapp -v "$$HOME/dockergo":/go -w /usr/src/myapp golang make dockerinstall
	#upx ~/dockergo/bin/${app}
	gzip -f ~/dockergo/bin/${app}

dockerinstall:
	go install -v -x -a -ldflags=${flags} ./...

targz:
	find . -name ".DS_Store" -delete
	find . -type f -name '\.*' -print
	cd .. && rm -f ${app}.tar.gz && tar czvf ${app}.tar.gz --exclude .git --exclude .idea ${app}

