package main

import (
	"log"
	"net/http"
	"os"

	"ytvault/internal/api"
)

func main() {
	addr := getenv("YTVAULT_ADDR", ":8080")
	dataPath := getenv("YTVAULT_DATA_PATH", "./data/state.json")

	app, err := api.NewApp(dataPath)
	if err != nil {
		log.Fatalf("init app: %v", err)
	}

	log.Printf("YtVault API listening on %s", addr)
	if err := http.ListenAndServe(addr, app.Routes()); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
