# syntax=docker/dockerfile:1

ARG DEBIAN_VER=buster
ARG GOLANG_VER=1.17.5-${DEBIAN_VER}
ARG PYTHON_VER=3.10.1-${DEBIAN_VER}
FROM golang:${GOLANG_VER} AS build

WORKDIR /app
COPY ./ ./

ARG GOPROXY=https://goproxy.cn
RUN go build -o webssh.exe main.go \
	&& cd lib \
	&& go build --buildmode=c-shared -o kailing_token/get-ip.so main.go

FROM python:${PYTHON_VER} AS setup

WORKDIR /app
COPY --from=build /app/lib ./token

RUN pip install -i https://pypi.tuna.tsinghua.edu.cn/simple build \
	&& cd token && python -m build

#######
# ssh #
#######
FROM debian:${DEBIAN_VER} AS ssh
RUN apt-get update; apt-get install tini
ENTRYPOINT ["/usr/bin/tini", "--"]

WORKDIR /

COPY --from=build /app/webssh.exe ./webssh

ENV NACOS_SERVER_IP=
ENV NACOS_SERVER_PORT=8848
ENV NACOS_SERVER_USERNAME=
ENV NACOS_SERVER_PASSWORD=
ENV AGENT_CIDR=

EXPOSE 80/tcp

CMD ["/webssh"]


#######
# vnc #
#######
FROM python:${PYTHON_VER} AS vnc
RUN apt-get update; apt-get install tini
ENTRYPOINT ["/usr/bin/tini", "--"]

WORKDIR /
COPY --from=setup /app/token/dist/kailing_token-*-py3-none-linux_x86_64.whl ./

RUN pip install -i https://pypi.tuna.tsinghua.edu.cn/simple websockify /kailing_token-*.whl

ENV NACOS_SERVER_IP=
ENV NACOS_SERVER_PORT=8848
ENV NACOS_SERVER_USERNAME=
ENV NACOS_SERVER_PASSWORD=
ENV AGENT_CIDR=

EXPOSE 80/tcp

CMD ["websockify", "80", "--token-plugin=kailing_token.token.Token"]
