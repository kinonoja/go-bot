package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/kinonoja/gobot/services/runner/internal/handler"
)

func main() {
	port := flag.Int("port", 0, "")
	maxConcurrent := flag.Int("max-concurrent", 5, "")
	flag.Parse()

	if *port == 0 {
		if p := os.Getenv("RUNNER_PORT"); p != "" {
			fmt.Sscanf(p, "%d", port)
		} else {
			*port = 8080
		}
	}

	h := handler.New(*maxConcurrent)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("runner listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
