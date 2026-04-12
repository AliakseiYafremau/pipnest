//go:build !linux && !darwin
// +build !linux,!darwin

package main

import "fmt"

func main() {
	fmt.Println("pipnest is currently supported on Linux and macOS.")
}
