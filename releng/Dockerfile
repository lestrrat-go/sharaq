FROM lestrrat-goxc:go1.4

RUN apt-get update && \
    apt-get install -y mercurial && \
    apt-get clean && \
    rm -rf /var/cache/apt/archives/* /var/lib/apt/lists/* 

COPY config.sh /
COPY build-on-docker.sh /
COPY release-on-docker.sh /

VOLUME ["/work/src/github.com/Peatix/sharaq"]

CMD echo "sharaq-docker built for go version 1.4"