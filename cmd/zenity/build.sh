#!/bin/bash

TAG=$(git tag --sort=-version:refname | head -n 1)

go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo

GOOS=windows GOARCH=386 go build -ldflags="-s -w -X main.tag=$TAG" -trimpath &&
zip -9 zenity_win32.zip zenity.exe &&
rm zenity.exe

GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X main.tag=$TAG" -trimpath &&
zip -9 zenity_win64.zip zenity.exe &&
rm zenity.exe

rm resource.syso

GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X main.tag=$TAG" -trimpath -o zenity_macos_x64 &&
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X main.tag=$TAG" -trimpath -o zenity_macos_arm &&
go run github.com/randall77/makefat zenity zenity_macos_x64 zenity_macos_arm &&
zip -9 zenity_macos.zip zenity &&
rm zenity zenity_macos_* 

go build -tags dev
