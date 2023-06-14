FROM golang:1.20.4-bullseye AS build-env

WORKDIR /root

RUN apt-get update -y
RUN apt-get install git jq wget -y

COPY . .

# RUN git clone https://github.com/cosmos/relayer.git && cd relayer && git checkout v2.3.1 && make build

RUN make build

FROM golang:1.20.4-bullseye
RUN apt-get update -y
RUN apt-get install ca-certificates jq wget -y
RUN wget -qO /usr/local/bin/yq https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64 && chmod a+x /usr/local/bin/yq

WORKDIR /root

COPY --from=build-env /root/build/rly /usr/bin/rly
COPY --from=build-env /root/rly/start-rly.sh /root/start-rly.sh
COPY --from=build-env /root/rly/mnemo.file /root/mnemo.file
RUN mkdir -p /root/tmp
COPY --from=build-env /root/rly/sevm_111-1.json /root/tmp/
COPY --from=build-env /root/rly/sevm_111-2.json /root/tmp/

RUN chmod -R 755 /root/start-rly.sh

EXPOSE 26656 26657 1317 9090 8545 8546

CMD ["bash","/root/start-rly.sh"]
