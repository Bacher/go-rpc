#!/usr/bin/env sh

cd protocol && protoc --go_out=. *.proto
