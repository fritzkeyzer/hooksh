test:
    go build ./...
    go test -v ./...

# run various examples against this project, saving output to demo/
demo:
    ./demo/generate.sh
