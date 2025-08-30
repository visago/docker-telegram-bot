BINARY := docker-telegram-bot

all:    lint build

build:
	go build -o ./${BINARY} ${VERSION_FLAGS} 

lint:
	gofmt -w *.go

clean:
	rm -rf ./${BINARY}

docker:
	docker build -t visago/docker-telegram-bot:latest .
	