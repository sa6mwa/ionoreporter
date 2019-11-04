DXHOST = fqdn.to.dx.host
VERSION = 1.0
GO = CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go

.PHONY: all clean full-clean build dependencies docker docker-run docker-to-dxhost run

all: clean build docker

clean:
	rm -f ionoreporter
full-clean: clean
	rm -rf output/

build: clean dependencies ionoreporter

dependencies:
	$(GO) get -v -d ./...

ionoreporter:
	$(GO) build -v -o ionoreporter -ldflags "-X main.version=$(VERSION)" ./...

docker:
	docker build -t ionoreporter:$(VERSION) .

docker-run: output
	docker run -u $(shell id -u):$(shell id -g) -ti --rm -v ${PWD}/output:/destination -e OUTDIR=/destination ionoreporter:$(VERSION)
output:
	mkdir output

docker-to-dxhost:
	# Maybe you don't have a remote registry to store your image, transfer it...
	docker save ionoreporter:$(VERSION) | bzip2 | pv | ssh $(DXHOST) 'bunzip2 > ionoreporter.tar'
	#ssh $(DXHOST) 'docker load -i ionoreporter.tar'

run: ionoreporter output
	OUTDIR=output ./ionoreporter
