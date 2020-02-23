NAME = ionoreporter
VERSION = 2.0.0
GOOS = $(shell uname -s | tr '[:upper:]' '[:lower:]')
GOARCH = amd64
GO = GOOS=$(GOOS) GOARCH=$(GOARCH) go
MODULE = github.com/sa6mwa/ionoreporter
SRC = ionoreporter.go
BINDIR = bin
OUTPUT = $(BINDIR)/$(NAME)
OUTDIR = output
DESTDIR = /usr/local/bin
SECRETENV = ~/.ionoreporter.config
DXHOST = fqdn.to.dx.host
SSHOPTS = -oProxyJump=jumphost

.PHONY: all tesseract clean full-clean build dependencies docker docker-run docker-to-dxhost run

all: clean build

clean:
	rm -f $(OUTPUT)
	test -d $(BINDIR) && rmdir $(BINDIR) || true
full-clean: clean
	rm -rf $(OUTDIR)/

build: tesseract clean dependencies $(OUTPUT)

tesseract:
	# Hint: You need tesseract-ocr and libtesseract-dev
	# E.g: apt-get install tesseract-ocr libtesseract-dev
	# Alpine: apk add tesseract-ocr tesseract-ocr-dev
dependencies:
	#$(GO) get -v -d ./...
	$(GO) mod download

$(BINDIR):
	mkdir $(BINDIR)
$(OUTPUT): $(BINDIR)
	#$(GO) build -v -o $(OUTPUT) -ldflags '-extldflags "-static -l jpeg" -X main.version=$(VERSION)' ./...
	$(GO) build -v -o $(OUTPUT) -ldflags '-X main.version=$(VERSION)' ./...
	ldd $(OUTPUT)
go.mod:
	go mod init $(MODULE)
	go mod tidy

install: $(OUTPUT)
	install $(OUTPUT) $(DESTDIR)


docker: $(OUTPUT)
	docker build -t $(NAME):$(VERSION) .

docker-run: $(OUTDIR)
	docker run -u $(shell id -u):$(shell id -g) -ti --rm \
	-v ${PWD}/$(OUTDIR):/destination \
	-e IRPT_OUTDIR=/destination \
	-e IRPT_PUSH_FREQUENCY=2 \
	-e IRPT_INTERVAL=15m \
	--env-file $(SECRETENV) \
	$(NAME):$(VERSION)

$(OUTDIR):
	mkdir $(OUTDIR)

docker-to-dxhost:
	docker save $(NAME):$(VERSION) | bzip2 | pv | ssh $(SSHOPTS) $(DXHOST) 'bunzip2 > $(NAME).tar'
	ssh $(SSHOPTS) $(DXHOST) 'sudo docker load -i $(NAME).tar'

docker-prune-dxhost:
	ssh $(SSHOPTS) $(DXHOST) 'sudo docker image prune -f'

run: $(OUTPUT) $(OUTDIR)
	IRPT_SLACK=false IRPT_OUTDIR=$(OUTDIR) $(OUTPUT)
