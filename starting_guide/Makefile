.PHONY: prepare-bin
prepare-bin:
	rm -rf ./bin || true
	mkdir -p ./bin || true

.PHONY: build-linux
build-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/linux_amd64/starting_guide ./cmd/starting_guide

.PHONY: build-darwin
build-darwin:
	GOOS=darwin GOARCH=amd64 go build -o bin/darwin_amd64/starting_guide ./cmd/starting_guide

.PHONY: build-windows
build-windows:
	GOOS=windows GOARCH=amd64 go build -o bin/windows_amd64/starting_guide ./cmd/starting_guide

.PHONY: build
build: \
	prepare-bin \
	build-linux \
	build-darwin \
	build-windows

.PHONY: release
release:
	rm -rf ./release || true
	mkdir -p ./release || true
	zip release/starting_guide_linux_amd64.zip ./bin/linux_amd64/starting_guide
	zip release/starting_guide_darwin_amd64.zip ./bin/darwin_amd64/starting_guide
	zip release/starting_guide_windows_amd64.zip ./bin/windows_amd64/starting_guide
