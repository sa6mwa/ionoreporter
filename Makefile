NAME = ionoreporter
VERSION = 3.2.0
GOOS = $(shell uname -s | tr '[:upper:]' '[:lower:]')
GOARCH = amd64
GO = GOOS=$(GOOS) GOARCH=$(GOARCH) go
GODOCKER = GOOS=linux GOARCH=amd64 go
MODULE = github.com/sa6mwa/ionoreporter
SRC = $(MODULE)/cmd/ionoreporter
BINDIR = bin
OUTPUT = $(BINDIR)/$(NAME)
OUTPUTDOCKERBIN = $(BINDIR)/$(NAME)-docker-bin
OUTDIR = output
DBFILE = ionize.db
DESTDIR = /usr/local/bin
SECRETENV = ~/.ionoreporter.config
DXUSER = 1001
DXGROUP = 1001
VOLUME = /storage/ionoreporter
DXENVFILE = /root/.ionoreporter.config
DXHOST = fqdn.to.dx.host
SSHOPTS =
#SSHOPTS = -oProxyJump=jumphost

.PHONY: all tesseract clean full-clean build dependencies docker dockerold docker-run docker-to-dxhost docker-deploy-to-dxhost docker-redeploy-to-dxhost run

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
	$(GO) get -v -d ./...

$(BINDIR):
	mkdir $(BINDIR)
$(OUTPUT): $(BINDIR)
	$(GO) build -v -ldflags '-X main.version=$(VERSION)' -o $(OUTPUT) $(SRC)

$(OUTPUTDOCKERBIN): $(BINDIR)
	$(GODOCKER) build -v -ldflags '-X main.version=$(VERSION)' -o $(OUTPUTDOCKERBIN) $(SRC)

go.mod:
	go mod init $(MODULE)
	go mod tidy

install: $(OUTPUT)
	install $(OUTPUT) $(DESTDIR)

dockerold: $(OUTPUTDOCKERBIN)
	docker build -t $(NAME):$(VERSION) --build-arg VERSION=$(VERSION) -f Dockerfile.old .

docker:
	docker build -t $(NAME):$(VERSION) --build-arg VERSION=$(VERSION) -f Dockerfile .

docker-run: $(OUTDIR)
	docker run -u $(shell id -u):$(shell id -g) -ti --rm \
	-v ${PWD}/$(OUTDIR):/destination \
        -e DBFILE=/destination/$(DBFILE) \
        -e DISCORD=true -e DAILY=true -e FREQUENT=false \
	--env-file $(SECRETENV) \
	$(NAME):$(VERSION)

$(OUTDIR):
	mkdir $(OUTDIR)

docker-to-dxhost:
	docker save $(NAME):$(VERSION) | bzip2 | ssh $(SSHOPTS) $(DXHOST) 'bunzip2 > $(NAME).tar'
	ssh $(SSHOPTS) $(DXHOST) 'sudo docker load -i $(NAME).tar'

docker-prune-dxhost:
	ssh $(SSHOPTS) $(DXHOST) 'sudo docker image prune -f'

docker-run-on-dxhost:
	ssh $(SSHOPTS) $(DXHOST) 'sudo docker run -d -u $(DXUSER):$(DXGROUP) --name $(NAME) --restart unless-stopped -v $(VOLUME):/destination -e DBFILE=/destination/$(DBFILE) -e DISCORD=true -e DAILY=true -e FREQUENT=false --env-file $(DXENVFILE) $(NAME):$(VERSION)'

docker-stop-on-dxhost:
	ssh $(SSHOPTS) $(DXHOST) 'sudo docker stop $(NAME) && sudo docker container rm $(NAME)'

docker-deploy-to-dxhost: docker-to-dxhost docker-prune-dxhost docker-run-on-dxhost
docker-redeploy-to-dxhost: docker-stop-on-dxhost docker-deploy-to-dxhost

run: $(OUTPUT) $(OUTDIR)
	DBFILE=$(OUTDIR)/$(DBFILE) $(OUTPUT)
