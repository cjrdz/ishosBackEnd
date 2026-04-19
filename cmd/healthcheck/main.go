package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	url := flag.String("url", "http://127.0.0.1:8080/health", "health endpoint URL")
	timeout := flag.Duration("timeout", 3*time.Second, "HTTP request timeout")
	flag.Parse()

	client := &http.Client{Timeout: *timeout}
	resp, err := client.Get(*url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck failed: %v\n", err)
		os.Exit(1)
	}

	code := resp.StatusCode
	_ = resp.Body.Close()

	if code < 200 || code >= 400 {
		fmt.Fprintf(os.Stderr, "healthcheck unhealthy status: %d\n", code)
		os.Exit(1)
	}
}
