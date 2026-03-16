.PHONY: build test lint fmt vet clean install

build:
	go build -o bin/inferctl .

install: build
	cp bin/inferctl /usr/local/bin/

test:
	go test ./... -v -count=1

lint:
	golangci-lint run

fmt:
	go fmt ./...

vet:
	go vet ./...

clean:
	rm -rf bin/ k8s/

example-gen:
	go run . gen -f examples/model.yaml -o /tmp/inferctl-example
	@echo "---"
	@cat /tmp/inferctl-example/*.yaml
