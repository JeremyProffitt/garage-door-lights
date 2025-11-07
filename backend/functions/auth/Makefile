.PHONY: build-AuthFunction

build-AuthFunction:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap main.go
	chmod +x bootstrap
