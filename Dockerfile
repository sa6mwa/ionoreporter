FROM busybox:latest
ARG HOME=/ionoreporter
ENV HOME $HOME
WORKDIR $HOME
copy ionoreporter $HOME
CMD ["./ionoreporter"]
