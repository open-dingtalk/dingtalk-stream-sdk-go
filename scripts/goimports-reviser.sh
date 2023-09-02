#!/bin/bash

set -e

go install github.com/incu6us/goimports-reviser/v2@latest

PROJECT_NAME=github.com/open-dingtalk/dingtalk-stream-sdk-go

find . -name '*.go' -print0 | while IFS= read -r -d '' file; do
  goimports-reviser -file-path "$file" -project-name "$PROJECT_NAME"
done
