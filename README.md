# pipnest

pipnest is a terminal-based Python dependency manager with a clean interactive UI.
It helps you search, install, and manage packages inside virtual environments.

## Supported Platforms

This project is intentionally supported only on:

- Linux
- macOS (Darwin)

Build constraints are enforced in source files, and unsupported systems fail compilation.

## Quickstart

Install with Go:

```bash
go install github.com/Rotlerxd/pipnest@latest
```

Then run:

```bash
pipnest
```

## Binary Download

Download the release binary:

```
curl -L https://github.com/Rotlerxd/pipnest/releases/download/v0.0.1/pipnest -o pipnest
```

Make it executable:

```
chmod +x pipnest
```

Move it to your PATH (optional, so you can run it without `./`):

```
sudo mv pipnest /usr/local/bin/pipnest
```

Run:

```
pipnest
```

## Usage Example

Run pipnest and use the package search flow:

```bash
pipnest
# Type a package name (for example: requests)
# Press Enter to search
# Use arrow keys to inspect package details
```

## Development

Run tests:

```bash
go test ./...
```

Verify OS-restricted builds:

```bash
GOOS=linux GOARCH=amd64 go build ./...
GOOS=darwin GOARCH=arm64 go build ./...
GOOS=windows GOARCH=amd64 go build ./...
```

The Windows build should fail by design.

## Publish Checklist

```bash
go mod tidy
go test ./...
git tag v0.1.0
git push origin main --tags
```

Then users can install with:

```bash
go install github.com/Rotlerxd/pipnest@latest
```
