
all:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o getimg main.go

clean: getimg
	rm getimg