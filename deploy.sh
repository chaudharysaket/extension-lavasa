#!/usr/bin/env bash

set -Eeuo pipefail

echo "Remove the bin directory if it exists"
[ -d "bin" ] && rm -r bin
GOOS=linux GOARCH=amd64 go build -o bin/extensions/extension-lavasa main.go
chmod +x bin/extensions/extension-lavasa

cd bin
zip -r extension.zip extensions/

aws lambda publish-layer-version \
 --layer-name "extension-lavasa" \
 --region "us-west-2" \
 --zip-file  "fileb://extension.zip"