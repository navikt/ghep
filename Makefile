.PHONY: build check staticcheck vulncheck deadcode fmt test vet

run:
	go run main.go

check: staticcheck vulncheck deadcode vet fmt

staticcheck:
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...

vulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

deadcode:
	go run golang.org/x/tools/cmd/deadcode@latest -test ./...

generate:
	go generate ./...

vet:
	go vet ./...

fmt:
	go run mvdan.cc/gofumpt@latest -w ./

test: check
	go test ./...
