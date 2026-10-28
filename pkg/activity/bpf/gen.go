//go:build bpfgen

package bpf

// bpf2go reads -tags in the legacy "// +build" syntax: "linux,amd64 linux,arm64"
// means linux && (amd64 || arm64), the constraint of tracker_linux.go.
//go:generate go tool -modfile=../../../tools.mod bpf2go -target bpfel -tags "linux,amd64 linux,arm64" counter counter.c -- -O2 -g -Wall -I/usr/include/x86_64-linux-gnu -I/usr/include/aarch64-linux-gnu
