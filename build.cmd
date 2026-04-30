@echo off
mkdir bin 2>nul
go build -o bin\cloud.exe ./cmd/cloud && echo built bin\cloud.exe
go build -o bin\home.exe ./cmd/home && echo built bin\home.exe
