VERSION = v0.6.0x
LDFLAGS = -ldflags "-X uhppote.VERSION=$(VERSION)" 
CMD     = ./bin/uhppoted-acl-s3
DIST   ?= development
DEBUG  ?= --debug

all: test      \
	 benchmark \
     coverage

clean:
	go clean
	rm -rf bin

format: 
	go fmt ./...

build: format
	mkdir -p bin
	go build -o bin ./...

test: build
	go test ./...

vet: build
	go vet ./...

lint: build
	golint ./...

benchmark: build
	go test -bench ./...

coverage: build
	go test -cover ./...

release: test vet
	mkdir -p dist/$(DIST)/windows
	mkdir -p dist/$(DIST)/darwin
	mkdir -p dist/$(DIST)/linux
	mkdir -p dist/$(DIST)/arm7
	env GOOS=linux   GOARCH=amd64         go build -o dist/$(DIST)/linux   ./...
	env GOOS=linux   GOARCH=arm   GOARM=7 go build -o dist/$(DIST)/arm7    ./...
	env GOOS=darwin  GOARCH=amd64         go build -o dist/$(DIST)/darwin  ./...
	env GOOS=windows GOARCH=amd64         go build -o dist/$(DIST)/windows ./...

release-tar: release
	find . -name ".DS_Store" -delete
	tar --directory=dist --exclude=".DS_Store" -cvzf dist/$(DIST).tar.gz $(DIST)
	cd dist; zip --recurse-paths $(DIST).zip $(DIST)

debug: build
	$(CMD) store-acl --no-log --key ../runtime/acl/uhppoted --url "http://localhost:8080/QWERTY54.tar.gz"

usage: build
	$(CMD)

help: build
	$(CMD) help
	$(CMD) help load-acl

version: build
	$(CMD) version

load-acl-http: build
	$(CMD) load-acl --keys ../runtime/acl --url "https://github.com/uhppoted/uhppoted/blob/master/runtime/simulation/QWERTY54.tar.gz?raw=true"

load-acl-s3: build
	$(CMD) load-acl --keys ../runtime/acl --credentials "../runtime/.credentials.test" --url "s3://uhppoted-test/simulation/QWERTY54.tar.gz"

store-acl: build
	$(CMD) help store-acl
	$(CMD) get-acl
