.PHONY: build cluster test

default: build

build:
	go build -o bin/swarm    github.com/InsighterInc/bxmp/cmd/swarm
	go build -o bin/geth     github.com/InsighterInc/bxmp/cmd/geth
	go build -o bin/bootnode github.com/InsighterInc/bxmp/cmd/bootnode

cluster: build
	scripts/boot-cluster.sh

test:
	go test -v github.com/InsighterInc/bxmp/swarm/...
