FROM golang:1.15 as golangbuilder
ARG VERSION=3.2.0
ENV VERSION=${VERSION}
LABEL stage=intermediate
WORKDIR /ionoreporter
COPY . .
RUN mkdir -p bin/
RUN apt-get update \
  && apt-get install -y tesseract-ocr libtesseract-dev \
  && rm -rf /var/lib/apt/lists/*
RUN GOOS=linux GOARCH=amd64 go build -v -ldflags "-X main.version=${VERSION}" -o bin/ionoreporter github.com/sa6mwa/ionoreporter/cmd/ionoreporter 2>&1

FROM debian:10-slim
RUN apt-get update \
  && apt-get install -y tesseract-ocr \
  && rm -rf /var/lib/apt/lists/*
ARG HOME=/ionoreporter
ENV HOME $HOME
WORKDIR $HOME
COPY --from=golangbuilder /ionoreporter/bin/ionoreporter .
CMD ["./ionoreporter"]
