NAME=pub-sub
BINDIR=bin
VERSION=$(shell git describe --tags || echo "unknown version")
BUILDTIME=$(shell date -u)
GOBUILD=go build -ldflags '-X "github.com/ninomiyx/$(NAME)/constant.Version=$(VERSION)" \
		-X "github.com/ninomiyx/$(NAME)/constant.BuildTime=$(BUILDTIME)" \
		-w -s'

PLATFORM_LIST = \
	darwin-amd64 \
	linux-386 \
	linux-amd64 \
	linux-armv5 \
	linux-armv6 \
	linux-armv7 \
	linux-armv8 \
	linux-mips-softfloat \
	linux-mips-hardfloat \
	linux-mipsle \
	linux-mipsle-softfloat \
	linux-mips64 \
	linux-mips64le \
	freebsd-386 \
	freebsd-amd64

WINDOWS_ARCH_LIST = \
	windows-386 \
	windows-amd64

all: darwin-arm64 # linux-amd64 darwin-amd64

darwin-arm64:
	GOARCH=arm64 GOOS=darwin $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

darwin-amd64:
	GOARCH=amd64 GOOS=darwin $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-386:
	GOARCH=386 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-amd64:
	GOARCH=amd64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-armv5:
	GOARCH=arm GOOS=linux GOARM=5 $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-armv6:
	GOARCH=arm GOOS=linux GOARM=6 $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-armv7:
	GOARCH=arm GOOS=linux GOARM=7 $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-armv8:
	GOARCH=arm64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mips-softfloat:
	GOARCH=mips GOMIPS=softfloat GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mips-hardfloat:
	GOARCH=mips GOMIPS=hardfloat GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mipsle:
	GOARCH=mipsle GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mipsle-softfloat:
	GOARCH=mipsle GOMIPS=softfloat GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mips64:
	GOARCH=mips64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mips64le:
	GOARCH=mips64le GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

freebsd-386:
	GOARCH=386 GOOS=freebsd $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

freebsd-amd64:
	GOARCH=amd64 GOOS=freebsd $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

windows-386:
	GOARCH=386 GOOS=windows $(GOBUILD) -o $(BINDIR)/$(NAME)-$@.exe

windows-amd64:
	GOARCH=amd64 GOOS=windows $(GOBUILD) -o $(BINDIR)/$(NAME)-$@.exe

gz_releases=$(addsuffix .gz, $(PLATFORM_LIST))
zip_releases=$(addsuffix .zip, $(WINDOWS_ARCH_LIST))

$(gz_releases): %.gz : %
	chmod +x $(BINDIR)/$(NAME)-$(basename $@)
	gzip -f -S -$(VERSION).gz $(BINDIR)/$(NAME)-$(basename $@)

$(zip_releases): %.zip : %
	zip -m -j $(BINDIR)/$(NAME)-$(basename $@)-$(VERSION).zip $(BINDIR)/$(NAME)-$(basename $@).exe

all-arch: $(PLATFORM_LIST) $(WINDOWS_ARCH_LIST)

releases: $(gz_releases) $(zip_releases)

clean:
	rm -f $(BINDIR)/*

# Linux
DEV_ARCH=linux-amd64
# Mac M1
DEV_ARCH=darwin-arm64

server-1: $(DEV_ARCH)
	bin/pub-sub-$(DEV_ARCH) -mode=server -server-addr=127.0.0.1:5000 -peer-addrs=127.0.0.1:5000,127.0.0.1:5002,127.0.0.1:5003

server-2: $(DEV_ARCH)
	bin/pub-sub-$(DEV_ARCH) -mode=server -server-addr=127.0.0.1:5002 -peer-addrs=127.0.0.1:5000,127.0.0.1:5002,127.0.0.1:5003

server-3: $(DEV_ARCH)
	bin/pub-sub-$(DEV_ARCH) -mode=server -server-addr=127.0.0.1:5003 -peer-addrs=127.0.0.1:5000,127.0.0.1:5002,127.0.0.1:5003

client-1: $(DEV_ARCH)
	bin/pub-sub-$(DEV_ARCH) -mode=client -server-addr=127.0.0.1:5000

client-2: $(DEV_ARCH)
	bin/pub-sub-$(DEV_ARCH) -mode=client -server-addr=127.0.0.1:5002

client-3: $(DEV_ARCH)
	bin/pub-sub-$(DEV_ARCH) -mode=client -server-addr=127.0.0.1:5003
