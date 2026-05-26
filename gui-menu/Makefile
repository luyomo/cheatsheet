build:
	GOARCH=arm GOOS=linux CC=arm-linux-gnueabihf-gcc CGO_CFLAGS="-I/usr/arm-linux-gnueabihf/include" CGO_LDFLAGS="-L/usr/arm-linux-gnueabihf/lib -lmecab" go build -o bin/start-menu-service cmd/main.go

docker:
	docker build . -t emaxchou/menu-service:v0.0.2-$$(uname -m)
	docker push emaxchou/menu-service:v0.0.2-$$(uname -m)
