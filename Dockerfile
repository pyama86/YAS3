FROM golang:latest as builder
ADD . /opt/yas3
WORKDIR /opt/yas3/
ENV CGO_ENABLED=0
RUN GOOS=linux make build

FROM scratch
COPY --from=builder /opt/yas3/bin/yas3 /bin/yas3
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
CMD ["/bin/yas3"]
