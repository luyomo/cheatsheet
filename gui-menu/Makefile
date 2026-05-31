IMAGE_NAME   := emaxchou/menu-service
VERSION      := v0.0.2
PLATFORMS    := linux/amd64,linux/arm64
TARGET_BIN   := start-menu-service

build:
	GOARCH=arm GOOS=linux CC=arm-linux-gnueabihf-gcc CGO_CFLAGS="-I/usr/arm-linux-gnueabihf/include" CGO_LDFLAGS="-L/usr/arm-linux-gnueabihf/lib -lmecab" go build -o bin/start-menu-service cmd/main.go

docker:
	docker buildx build --platform $(PLATFORMS) -t $(IMAGE_NAME):$(VERSION) --push .
