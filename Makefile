
all:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o main main.go

clean: main
	rm main

build:
	gcloud builds submit --tag gcr.io/rellikcloudimage/image2cloud


deploy: 
	gcloud run deploy image2cloud \
     --image gcr.io/rellikcloudimage/image2cloud \
     --allow-unauthenticated