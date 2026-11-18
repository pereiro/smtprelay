#!/bin/sh
set -eu

cd "$(dirname "$0")"
output_dir="build/$(go env GOOS)/bin"
mkdir -p "$output_dir"
go build -mod=readonly -o "$output_dir/smtprelay" .
