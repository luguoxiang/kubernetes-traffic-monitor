# build stage
FROM golang:1.11-alpine AS build-env
RUN apk update
RUN apk add --no-cache gcc
RUN apk add musl-dev
RUN apk add libpcap
RUN apk add libpcap-dev
RUN apk add git
RUN apk add curl
ENV PROJECT_DIR /go/src/github.com/luguoxiang/kubernetes-traffic-monitor
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh |sh
RUN mkdir -p ${PROJECT_DIR}/cmd
ENV GOPATH /go
WORKDIR ${PROJECT_DIR}
ADD Gopkg.lock .
ADD Gopkg.toml .
RUN dep ensure -vendor-only -v
ADD cmd cmd
ADD pkg pkg
RUN go build -o traffic-monitor cmd/traffic-monitor/traffic-monitor.go

# final stage
FROM golang:1.11-alpine
RUN apk update
RUN apk add libpcap
RUN apk add tcpdump
WORKDIR /app
COPY --from=build-env /go/src/github.com/luguoxiang/kubernetes-traffic-monitor/traffic-monitor /app/
