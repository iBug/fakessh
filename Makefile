BIN = fakessh

.PHONY: all install connect

all: $(BIN)

$(BIN): $(wildcard *.go) go.mod go.sum
	go build -ldflags="-s -w"

install:
	install -Dm755 $(BIN) /usr/local/sbin/

connect:
	ssh -o ControlPath=none -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -p 22 root@127.0.0.1
