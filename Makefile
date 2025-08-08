BIN = fakessh

.PHONY: all install restart connect

all: $(BIN)

$(BIN): $(wildcard *.go) go.mod go.sum system_prompt.txt
	go build -ldflags="-s -w" -trimpath

install:
	install -Dm755 $(BIN) /usr/local/sbin/

restart:
	systemctl restart fakessh.service

connect:
	ssh -o ControlPath=none -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -p 22 root@127.0.0.1
