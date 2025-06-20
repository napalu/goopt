#!/bin/bash

# Example build script showing how to inject version information at build time
echo "Running go mod tidy on code"
echo ""
go mod tidy

#VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}
VERSION=${VERSION:-$(git tag --sort=-v:refname | head -n 1 || echo "dev")}
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

echo "Building version-demo..."
echo "  Version: $VERSION"
echo "  Commit: $COMMIT"
echo "  Build Time: $BUILD_TIME"

go build -ldflags "\
    -X main.Version=$VERSION \
    -X main.GitCommit=$COMMIT \
    -X main.BuildTime=$BUILD_TIME" \
    -o version-demo \
    main.go

echo "Build complete!"
echo ""
echo "Try running:"
echo "  ./version-demo --version"
echo "  ./version-demo --help"
echo "  ./version-demo server start -p 9000 --version"