BINARY_NAME=inaho-yamato

build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -v -o ${BINARY_NAME}

build-docker:
	docker build . -t inaho-yamato:latest