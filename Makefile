.PHONY: build build-cgo build-nocgo run dev install clean fmt

LDFLAGS = -ldflags="-s -w"
BIN = bin/happyboard

build:
	go build $(LDFLAGS) -o $(BIN) ./cmd/happyboard

build-cgo:
	CGO_ENABLED=1 go build $(LDFLAGS) -o $(BIN) ./cmd/happyboard

build-nocgo:
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BIN) ./cmd/happyboard

run: build
	@./$(BIN) -config config.yaml

dev:
	CGO_ENABLED=0 go run ./cmd/happyboard -config config.yaml

install: build
	systemctl --user disable --now happyboard 2>/dev/null || true
	sudo install -m 755 $(BIN) /usr/local/bin/happyboard
	rm -rf $(BIN)
	mkdir -p ~/.config/happyboard
	cp -f config.yaml ~/.config/happyboard/config.yaml
	mkdir -p ~/.config/systemd/user
	cp -f happyboard.service ~/.config/systemd/user/happyboard.service
	systemctl --user daemon-reload
	systemctl --user enable --now happyboard
	@echo "happyboard installed"

clean:
	rm -rf bin/

fmt:
	go fmt ./... && go vet ./...
