FROM busybox
MAINTAINER Łukasz Kurowski <crackcomm@gmail.com>
COPY ./dist/tdc /tdc
ENTRYPOINT ["/tdc"]
