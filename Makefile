.PHONY: build run demo test clean

# Build the gox CLI
build:
	go build -o gox ./cmd/gox

# Run tests
test:
	go test ./...

# Run demo
demo: build
	./gox run ./demo/

# Shorthand for demo
run: demo

# Clean build artifacts
clean:
	rm -f gox
