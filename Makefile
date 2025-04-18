CMD   = ./bin/uhppoted-app-s3
DIST  ?= development
DEBUG ?= --debug

.PHONY: clean
.PHONY: update
.PHONY: update-release

all: test      \
	 benchmark \
     coverage

clean:
	go clean
	rm -rf bin

update:
	go get -u github.com/uhppoted/uhppote-core@main
	go get -u github.com/uhppoted/uhppoted-lib@main
	go mod tidy

update-release:
	go get -u github.com/uhppoted/uhppote-core
	go get -u github.com/uhppoted/uhppoted-lib
	go mod tidy

update-all:
	go get -u github.com/uhppoted/uhppote-core
	go get -u github.com/uhppoted/uhppoted-lib
	go get -u github.com/aws/aws-sdk-go
	go get -u golang.org/x/sys
	go get -u golang.org/x/crypto
	go mod tidy

format: 
	go fmt ./...

build: format
	mkdir -p bin
	go build -trimpath -o bin ./...

test: build
	go test ./...

benchmark: build
	go test -bench ./...

coverage: build
	go test -cover ./...

vet: build
	go vet ./...

lint: build
	env GOOS=darwin  GOARCH=amd64 staticcheck ./...
	env GOOS=linux   GOARCH=amd64 staticcheck ./...
	env GOOS=windows GOARCH=amd64 staticcheck ./...

vuln:
	govulncheck ./...

build-all: build test vet lint
	mkdir -p dist/$(DIST)/linux
	mkdir -p dist/$(DIST)/arm
	mkdir -p dist/$(DIST)/arm7
	mkdir -p dist/$(DIST)/arm6
	mkdir -p dist/$(DIST)/darwin-x64
	mkdir -p dist/$(DIST)/darwin-arm64
	mkdir -p dist/$(DIST)/windows
	env GOOS=linux   GOARCH=amd64         GOWORK=off go build -trimpath -o dist/$(DIST)/linux        ./...
	env GOOS=linux   GOARCH=arm64         GOWORK=off go build -trimpath -o dist/$(DIST)/arm          ./...
	env GOOS=linux   GOARCH=arm   GOARM=7 GOWORK=off go build -trimpath -o dist/$(DIST)/arm7         ./...
	env GOOS=linux   GOARCH=arm   GOARM=6 GOWORK=off go build -trimpath -o dist/$(DIST)/arm6         ./...
	env GOOS=darwin  GOARCH=amd64         GOWORK=off go build -trimpath -o dist/$(DIST)/darwin-x64   ./...
	env GOOS=darwin  GOARCH=arm64         GOWORK=off go build -trimpath -o dist/$(DIST)/darwin-arm64 ./...
	env GOOS=windows GOARCH=amd64         GOWORK=off go build -trimpath -o dist/$(DIST)/windows      ./...

release: update-release build-all
	find . -name ".DS_Store" -delete
	tar --directory=dist/$(DIST)/linux        --exclude=".DS_Store" -cvzf dist/$(DIST)-linux-x64.tar.gz    .
	tar --directory=dist/$(DIST)/arm          --exclude=".DS_Store" -cvzf dist/$(DIST)-arm-x64.tar.gz      .
	tar --directory=dist/$(DIST)/arm7         --exclude=".DS_Store" -cvzf dist/$(DIST)-arm7.tar.gz         .
	tar --directory=dist/$(DIST)/arm6         --exclude=".DS_Store" -cvzf dist/$(DIST)-arm6.tar.gz         .
	tar --directory=dist/$(DIST)/darwin-x64   --exclude=".DS_Store" -cvzf dist/$(DIST)-darwin-x64.tar.gz   .
	tar --directory=dist/$(DIST)/darwin-arm64 --exclude=".DS_Store" -cvzf dist/$(DIST)-darwin-arm64.tar.gz .
	cd dist/$(DIST)/windows && zip --recurse-paths ../../$(DIST)-windows-x64.zip . -x ".DS_Store"

publish: release
	echo "Releasing version $(VERSION)"
	gh release create "$(VERSION)" "./dist/$(DIST)-arm-x64.tar.gz"      \
	                               "./dist/$(DIST)-arm7.tar.gz"         \
	                               "./dist/$(DIST)-arm6.tar.gz"         \
	                               "./dist/$(DIST)-darwin-arm64.tar.gz" \
	                               "./dist/$(DIST)-darwin-x64.tar.gz"   \
	                               "./dist/$(DIST)-linux-x64.tar.gz"    \
	                               "./dist/$(DIST)-windows-x64.zip"     \
	                               --draft --prerelease --title "$(VERSION)-beta" --notes-file release-notes.md

debug: build
	$(CMD) --debug load-acl  \
	       --dry-run \
	       --strict  \
	       --keys ../runtime/acl \
	       --credentials "../runtime/.credentials.test" \
	       --url "s3://uhppoted-test/simulation/QWERTY54.tar.gz" \

godoc:
	godoc -http=:80	-index_interval=60s

usage: build
	$(CMD)

help: build
	$(CMD) help
	$(CMD) help load-acl
	$(CMD) help store-acl
	$(CMD) help compare-acl

version: build
	$(CMD) version

load-acl-http: build
	$(CMD) load-acl --keys ../runtime/acl --url "https://github.com/uhppoted/uhppoted/blob/master/runtime/simulation/QWERTY54.tar.gz?raw=true"

load-acl-s3: build
	$(CMD) load-acl --strict --keys ../runtime/acl --credentials "../runtime/.credentials.test" --url "s3://uhppoted-test/simulation/QWERTY54.tar.gz"

load-acl-file: build
	$(CMD) load-acl --no-log --keys ../runtime/s3/keys --credentials "../runtime/.credentials.test" --url "file://../runtime/s3/hogwarts.tar.gz"

load-acl-file-with-pin: build
	$(CMD) load-acl --with-pin --no-log --keys ../runtime/s3/keys --credentials "../runtime/.credentials.test" --url "file://../runtime/s3/hogwarts-with-pin.tar.gz"

load-acl-zip: build
	$(CMD) load-acl --no-log --keys ../runtime/acl --credentials "../runtime/.credentials.test" --url "file://../runtime/simulation/QWERTY54.zip"

store-acl-http: build
	$(CMD) store-acl --key ../runtime/acl/uhppoted --url "http://localhost:8080/upload/uhppoted.tar.gz"

store-acl-s3: build
	$(CMD) store-acl --no-log --key ../runtime/acl/uhppoted --credentials "../runtime/.credentials.test" --url "s3://uhppoted-test/simulation/uhppoted.tar.gz"

store-acl-file: build
	$(CMD) store-acl --no-log --key ../runtime/acl/uhppoted --credentials "../runtime/.credentials.test" --url "file://../runtime/s3/uhppoted.tar.gz"

store-acl-file-with-pin: build
	$(CMD) store-acl --with-pin --no-log --key ../runtime/acl/uhppoted --credentials "../runtime/.credentials.test" --url "file://../runtime/s3/uhppoted.tar.gz"

store-acl-zip: build
	$(CMD) store-acl --no-log --key ../runtime/acl/uhppoted --credentials "../runtime/.credentials.test" --url "file://../runtime/s3/uhppoted.zip"

compare-acl-http: build
	$(CMD) compare-acl \
	       --keys   ../runtime/acl \
	       --key    ../runtime/acl/uhppoted \
	       --acl    "https://github.com/uhppoted/uhppoted/blob/master/runtime/simulation/QWERTY54.tar.gz?raw=true" \
	       --report "http://localhost:8080/upload/report.tar.gz"

compare-acl-s3: build
	$(CMD) compare-acl \
	       --credentials "../runtime/.credentials.test" \
	       --keys        ../runtime/acl \
	       --key         ../runtime/acl/uhppoted \
	       --acl         "s3://uhppoted-test/simulation/QWERTY54.tar.gz" \
	       --report      "s3://uhppoted-test/simulation/report.tar.gz"

compare-acl-file: build
	$(CMD) compare-acl \
           --no-log    \
	       --credentials "../runtime/.credentials.test"         \
	       --keys        ../runtime/s3/keys                     \
	       --key         ../runtime/acl/uhppoted                \
	       --acl         "file://../runtime/s3/hogwarts.tar.gz" \
	       --report      "file://../runtime/s3/report.tar.gz"

compare-acl-file-with-pin: build
	$(CMD) compare-acl --with-pin \
	       --credentials "../runtime/.credentials.test" \
	       --keys        ../runtime/s3/keys             \
	       --key         ../runtime/acl/uhppoted        \
	       --acl         "file://../runtime/s3/hogwarts-with-pin.tar.gz" \
	       --report      "file://../runtime/s3/report.tar.gz"

compare-acl-zip: build
	$(CMD) compare-acl \
	       --credentials "../runtime/.credentials.test" \
	       --keys        ../runtime/acl \
	       --key         ../runtime/acl/uhppoted \
	       --acl         "file://../runtime/simulation/QWERTY54.zip" \
	       --report      "file://../runtime/acl/report.zip"


