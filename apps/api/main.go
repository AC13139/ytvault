package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"ytvault/internal/api"
)

// Build-time version metadata. Populated via -ldflags:
//
//	go build -ldflags "-X main.Version=1.2.3 -X main.Commit=abc1234"
var (
	Version = "dev"
	Commit  = "unknown"
)

func main() {
	addr := getenv("YTVAULT_ADDR", ":8080")
	dataPath := getenv("YTVAULT_DATA_PATH", "./data/state.json")
	downloadDir := getenv("YTVAULT_DOWNLOAD_PATH", "./data/downloads")
	cookiesPath := getenv("YTVAULT_COOKIES_PATH", "./data/cookies.txt")

	// Setting both enables HTTP Basic authentication on every route
	// except /healthz. Leaving them unset keeps the API open, which is
	// only safe behind a trusted network or an authenticating proxy.
	authUser := getenv("YTVAULT_AUTH_USER", "")
	authPassword := getenv("YTVAULT_AUTH_PASSWORD", "")

	app, err := api.NewApp(api.Config{
		DataPath:     dataPath,
		DownloadDir:  downloadDir,
		CookiesPath:  cookiesPath,
		Version:      Version,
		Commit:       Commit,
		AuthUser:     authUser,
		AuthPassword: authPassword,
	})
	if err != nil {
		log.Fatalf("init app: %v", err)
	}

	if authUser != "" && authPassword != "" {
		log.Printf("HTTP Basic authentication enabled for user %q", authUser)
	} else if authUser != "" || authPassword != "" {
		log.Printf("warning: YTVAULT_AUTH_USER and YTVAULT_AUTH_PASSWORD must both be set; " +
			"authentication is disabled")
	}

	// Stop scheduler on shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("shutting down")
		app.Close()
		os.Exit(0)
	}()

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
