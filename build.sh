#!/bin/bash
rm -rf builds/*
echo building darwin
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o builds/darwin/signatize
upx -q builds/darwin/signatize >> /dev/null

echo building linux
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o builds/linux/signatize
upx -q builds/linux/signatize >> /dev/null

echo building windows
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o builds/windows/signatize.exe
upx -q builds/windows/signatize.exe >> /dev/null
