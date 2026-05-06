test:
    go build ./...
    go test -v ./...

install:
    go install github.com/fritzkeyzer/hooksh/cmd/hooksh

# run various examples against this project, saving output to demo/
demo: install
    ./demo/generate.sh
