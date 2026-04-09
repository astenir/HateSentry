package main

import (
	"hatesentry/internal/app"
	"log"
)

func main() {
	application := app.NewApp()
	if err := application.Run(); err != nil {
		log.Fatalf("Failed to run application: %v", err)
	}
}
