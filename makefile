.PHONY: web

web: 
	cd web && pnpm install && pnpm run build

bin:  
	# build arm64 mode 
	GOARCH=arm64 go build --ldflags "-s -w -X main.version=$(shell git describe --tags --always)"

	# build intel mode 
	GOARCH=amd64 go build -o wsterm-amd64 --ldflags "-s -w -X main.version=$(shell git describe --tags --always)"


	# build linux mode 
	GOOS=linux GOARCH=amd64 go build -o wsterm-linux-amd64 --ldflags "-s -w -X main.version=$(shell git describe --tags --always)"

all: web bin