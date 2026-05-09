package main

import (
	"log"

	"go-web-gin-health/internal/bootstrap"
)

func main() {
	app, err := bootstrap.NewApplication()
	if err != nil {
		log.Fatalf("bootstrap application failed: %v", err)
	}

	if err := app.Run(); err != nil {
		log.Fatalf("run server failed: %v", err)
	}
}
