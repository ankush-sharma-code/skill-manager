# Agent notes

## Build

```bash
go build ./...
```

## Test

```bash
go test -race ./...
```

## Lint

```bash
go vet ./...
golangci-lint run
```

CI uses Go 1.22, golangci-lint 2.15.0, and crap4go. The build fails when a CRAP score reaches 30.
