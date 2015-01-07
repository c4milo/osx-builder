CGO_ENABLED 	:= 1
CGO_CFLAGS		:=-I$(CURDIR)/vendor/libvix/include -Werror
CGO_LDFLAGS		:=-L$(CURDIR)/vendor/libvix -lvixAllProducts -ldl -lpthread

DYLD_LIBRARY_PATH	:=$(CURDIR)/vendor/libvix
LD_LIBRARY_PATH		:=$(CURDIR)/vendor/libvix

NAME 		:= go-osx-builder
VERSION 	:= v1.0.0
PLATFORM 	:= $(shell go env | grep GOHOSTOS | cut -d '"' -f 2)
ARCH 		:= $(shell go env | grep GOARCH | cut -d '"' -f 2)

export CGO_CFLAGS CGO_LDFLAGS DYLD_LIBRARY_PATH LD_LIBRARY_PATH CGO_ENABLED

build:
	go build -ldflags "-X main.Version $(VERSION)" -o build/$(NAME)

dist: build
	@rm -rf dist && mkdir dist
	@cp -r vendor/libvix build/libvix
	@echo "DYLD_LIBRARY_PATH=./libvix LD_LIBRARY_PATH=./libvix ./go-osx-builder" > build/run.sh
	(cd $(shell pwd)/build && tar -cvzf ../dist/$(NAME)_$(VERSION)_$(PLATFORM)_$(ARCH).tar.gz *); \
	(cd $(shell pwd)/dist && shasum -a 512 $(NAME)_$(VERSION)_$(PLATFORM)_$(ARCH).tar.gz > $(NAME)_$(VERSION)_$(PLATFORM)_$(ARCH).tar.gz.sha512);

deps:
	go get github.com/c4milo/github-release
	go get github.com/satori/go.uuid
	go get gopkg.in/unrolled/render.v1
	go get github.com/codegangsta/negroni
	go get github.com/meatballhat/negroni-logrus
	go get github.com/julienschmidt/httprouter
	go get github.com/Sirupsen/logrus
	go get github.com/c4milo/unzipit
	go get github.com/hooklift/govmx
	go get github.com/dustin/go-humanize
	go get github.com/stretchr/graceful
	mkdir -p $GOPATH/src/github.com/hooklift/govix && git clone https://github.com/hooklift/govix.git $GOPATH/src/github.com/hooklift/govix

release: dist
	@latest_tag=$$(git describe --tags `git rev-list --tags --max-count=1`); \
	comparison="$$latest_tag..HEAD"; \
	if [ -z "$$latest_tag" ]; then comparison=""; fi; \
	changelog=$$(git log $$comparison --oneline --no-merges --reverse); \
	github-release c4milo/$(NAME) $(VERSION) "$$(git rev-parse --abbrev-ref HEAD)" "**Changelog**<br/>$$changelog" 'dist/*'; \
	git pull

test:
	go test ./...

clean:
	go clean ./...

.PHONY: dist release build test install clean
