
all:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o getimg main.go

clean: getimg
	rm getimg

deploy:
	gcloud run deploy \
  	--add-cloudsql-instances rellikcloudimage:us-central1:image-sql \
  	--vpc-connector="quickstart-instance" --vpc-egress=all-traffic \
  	# --set-env-vars DB_NAME="image_db" \
  	# --set-env-vars DB_USER="sqlserver" \
  	# --set-env-vars DB_PASS="p@ssWord" \
  	# --set-env-vars INSTANCE_CONNECTION_NAME="rellikcloudimage:us-central1:image-sql" \
  	# --set-env-vars DB_PORT="1433" \
  	# --set-env-vars INSTANCE_HOST="10.21.0.3" \
  	# --set-env-vars DB_ROOT_CERT="certs/server-ca.pem" \
  	# --set-env-vars PRIVATE_IP="TRUE"