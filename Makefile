GIT_COMMIT=$(shell git rev-parse --verify HEAD)
UTC_NOW=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

.PHONY: build-dev
build-dev:
	CGO_ENABLED=0 go build \
		-ldflags="-X 'main.version=dev' -X 'main.commit=${GIT_COMMIT}' -X 'main.date=${UTC_NOW}'" \
		-o labctl

.PHONY: build-dev-darwin-arm64
build-dev-darwin-arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build \
		-ldflags="-X 'main.version=dev' -X 'main.commit=${GIT_COMMIT}' -X 'main.date=${UTC_NOW}'" \
		-o labctl

.PHONY: release
release:
	goreleaser --clean

.PHONY: release-snapshot
release-snapshot:
	goreleaser release --snapshot --clean

.PHONY: test-e2e
test-e2e:
	go test -v -count 1 ./e2e/exec
