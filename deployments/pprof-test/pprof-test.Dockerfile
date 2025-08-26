FROM alpine
WORKDIR /
COPY build/pprof-test-linux-amd64 /pprof-test
CMD ["/pprof-test"]
EXPOSE 8080
