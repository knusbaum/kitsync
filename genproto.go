package kitsync

//go:generate protoc --go_out=ksrpc --go_opt=paths=source_relative --go-grpc_out=ksrpc --go-grpc_opt=paths=source_relative kitsync.proto
