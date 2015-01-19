FROM lestrrat-goxc:go1.4

RUN apt-get update && \
    apt-get install -y mercurial && \
    apt-get clean && \
    rm -rf /var/cache/apt/archives/* /var/lib/apt/lists/* && \
    go get -u github.com/goamz/goamz/aws && \
    go get -u github.com/goamz/goamz/s3 && \
    go get -u github.com/disintegration/imaging && \
    go get -u github.com/bradfitz/gomemcache/memcache && \
    go get -u github.com/lestrrat/go-server-starter && \
    go get -u github.com/lestrrat/go-apache-logformat && \
    go get -u github.com/lestrrat/go-file-rotatelogs

COPY config.sh /
COPY build-on-docker.sh /
COPY release-on-docker.sh /

VOLUME ["/work/src/github.com/Peatix/sharaq"]

CMD echo "sharaq-docker built for go version 1.4"