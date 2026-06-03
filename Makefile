run:
	go run cmd/*.go
build:
	go build -o mydb ./cmd
clean: 
	rm -f mydb data.db
tidy:
	go mod tidy