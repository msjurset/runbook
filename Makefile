VERSION ?= dev
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o runbook ./cmd/runbook

run:
	go run ./cmd/runbook

test:
	go test -v ./...

release: clean test
	@mkdir -p dist
	cp runbook.1 dist/
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o dist/runbook ./cmd/runbook && \
		tar -czf dist/runbook-$(VERSION)-linux-amd64.tar.gz -C dist runbook runbook.1 && rm dist/runbook
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o dist/runbook ./cmd/runbook && \
		tar -czf dist/runbook-$(VERSION)-linux-arm64.tar.gz -C dist runbook runbook.1 && rm dist/runbook
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o dist/runbook ./cmd/runbook && \
		tar -czf dist/runbook-$(VERSION)-darwin-amd64.tar.gz -C dist runbook runbook.1 && rm dist/runbook
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o dist/runbook ./cmd/runbook && \
		tar -czf dist/runbook-$(VERSION)-darwin-arm64.tar.gz -C dist runbook runbook.1 && rm dist/runbook
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/runbook.exe ./cmd/runbook && \
		cd dist && zip runbook-$(VERSION)-windows-amd64.zip runbook.exe runbook.1 && rm runbook.exe
	rm dist/runbook.1

clean:
	rm -rf dist/
	rm -f runbook

deploy: build install-man install-completion
	cp runbook ~/.local/bin/

install-man:
	install -d /usr/local/share/man/man1
	install -m 644 runbook.1 /usr/local/share/man/man1/runbook.1

install-completion:
	install -d ~/.oh-my-zsh/custom/completions
	install -m 644 _runbook ~/.oh-my-zsh/custom/completions/_runbook

.PHONY: build run test release clean deploy install-man install-completion
