build:
	GOARCH=arm GOOS=linux CC=arm-linux-gnueabihf-gcc CGO_CFLAGS="-I/usr/arm-linux-gnueabihf/include" CGO_LDFLAGS="-L/usr/arm-linux-gnueabihf/lib -lmecab" go build -o bin/mysql-checker cmd/main.go
