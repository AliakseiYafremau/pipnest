package main

import (
	"context"
	"fmt"
	"log"
	"time"

	req "pipnest/internal/packages/manager"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	manager := req.NewUVManager("uv")

	output, err := manager.RunPython(ctx, "print(2 + 2)")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("uv manager smoke test")
	fmt.Println("python output:", output)
}
