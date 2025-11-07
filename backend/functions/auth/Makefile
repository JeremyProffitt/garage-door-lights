.PHONY: build-AuthFunction

build-AuthFunction:
	go mod tidy
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -mod=readonly -tags lambda.norpc -o bootstrap main.go
