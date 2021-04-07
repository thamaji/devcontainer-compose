#!/bin/bash
set -eu

go build -ldflags="-w -s" -trimpath -o docker-compose
