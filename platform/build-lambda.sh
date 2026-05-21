#!/bin/bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lambda/teams-notifier"

echo "Building teams-notifier Lambda..."
cd "$DIR"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bootstrap .
zip handler.zip bootstrap
rm bootstrap
echo "Built: $DIR/handler.zip"
