FROM debian:10-slim
RUN apt-get update \
  && apt-get install -y tesseract-ocr \
  && rm -rf /var/lib/apt/lists/*
ARG HOME=/ionoreporter
ENV HOME $HOME
WORKDIR $HOME
copy bin/ionoreporter $HOME
CMD ["./ionoreporter"]
