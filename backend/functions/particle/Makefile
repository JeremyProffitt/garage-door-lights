.PHONY: build-ParticleFunction

build-ParticleFunction:
	go mod download
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap main.go
