# ionoreporter

Ionoreporter v3 is a small app that download ionograms (images) from public
sources, interpret parameters written as text in the images (using OCR) and
populates a database with those parameters.

Parameters are foF2 (critical frequency of the ordinary wave reflected by the
F2 layer), foE (E layer), fmin (minimum reflected frequency), hmF2 (calculated
actual height of the F2 layer) and the optimal frequency range for NVIS
regional shortwave (HF/MF) communication.

These parameters are pushed as short daily text messages to a Discord or Slack
integration webhook URL. Version 3.0.0 feature only daily reports of parameters
from the previous 24 hours. Future versions will implement prediction and more
frequent push messages of current conditions.

Version 3 is a complete rewrite of previous versions and does not generate pdf
files anymore, see previous releases for that functionality.

The app is written in [Go](https://golang.org) and
builds with GNU Make (using a Makefile). The Makefile can also be used to build
a docker image to run or deploy `ionoreporter` as a container.

## Dependencies

Golang 1.14 (probably works with earlier too), Docker, GNU Make,
`tesseract-ocr` and `libtesseract-dev`. Install like this...

```bash
# Debian/Ubuntu
apt-get install tesseract-ocr libtesseract-dev
# Alpine
apk add tesseract-ocr tesseract-ocr-dev
# MacOS and Homebrew
brew install tesseract
```

## Simple installation

If you have `go` already installed, you can run `go get
github.com/sa6mwa/ionoreporter` and you will have the `ionoreporter` binary in
your `$GOPATH/bin`.

## Building and running

```bash
# See Makefile for more info.
# Default build env is amd64 and whatever uname -s says.
make all
make run
# If you have docker, you can build and run ionoreporter as a container:
make docker
make docker-run
```

## Building and deploying to a remote docker host

The `Makefile` has some simple automation for transferring the docker container you build
with `make docker` to a remote host called DXHOST using `ssh`. You need to specify the variables `SSHOPTS` and `DXHOST` to use these build steps, for example...

```bash
make docker
make DXHOST="user@myremote.host.provider.com" SSHOPTS="" docker-to-dxhost
# using a jumphost...
make DXHOST="user@final.host.tld" SSHOPTS="-oProxyJump=jumphost" docker-to-dxhost
# to start the image you probably also need to override DXUSER, DXGROUP,
# VOLUME, DXENVFILE
make DXHOST="remote.host.tld" SSHOPTS="" DXUSER=1003 DXGROUP=1003 VOLUME=/var/opt/storage DXENVFILE=/home/user/.ionoreporter.config docker-run-on-dxhost

# there are also docker-deploy-to-dxhost and docker-redeploy-to-dxhost that
# combine several steps: stopping, uploading, pruning containers & images and
# starting the container (override the same variables above if necessary)...

make DXHOST="remote.host.tld" SSHOPTS="" docker-deploy-to-dxhost
make DXHOST="remote.host.tld" SSHOPTS="" docker-redeploy-to-dxhost
```

## Other examples

```
$ make docker-run
docker run -u 501:20 -ti --rm \
        -v /home/sa6mwa/ionoreporter/output:/destination \
        -e DBFILE=/destination/ionize.db \
        -e DISCORD=true -e DAILY=true -e FREQUENT=false \
        --env-file ~/.ionoreporter.config \
        ionoreporter:3.0.0
{"level":"info","message":"Starting ionoreporter 3.0.0 with db /destination/ionize.db","timestamp":"2020-10-29T18:34:05Z"}
{"level":"info","message":"Scheduling scrape function with cronspec */15 * * * *","timestamp":"2020-10-29T18:34:05Z"}
{"level":"info","message":"Scheduling Slack and/or Discord daily reports with cronspec 0 5 * * *","timestamp":"2020-10-29T18:34:05Z"}
{"level":"info","message":"ionoreporter started successfully","timestamp":"2020-10-29T18:34:05Z"}
```
The file `~/.ionoreporter.config` need to define the `DAILY_DISCORDURL` environment variable...

```
DAILY_DISCORDURL=DAILY_DISCORDURL=https://discord.com/api/webhooks/key/key
```

