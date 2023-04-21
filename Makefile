
all:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o main main.go

clean: main
	rm main

deploy:
	gcloud run deploy \
  	--add-cloudsql-instances rellikcloudimage:us-central1:image-sql \
  	--vpc-connector="quickstart-instance" --vpc-egress=all-traffic \