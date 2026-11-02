package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// healthcheck asks the running server's /healthz whether it is up; the container's health check runs it, since the
// image has no shell or curl. It reads LH_HTTP_ADDR alone: config.Load needs settings the check does not use.
func healthcheck() error {
	addr := os.Getenv("LH_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	hostPort, err := healthcheckAddress(addr)
	if err != nil {
		return err
	}
	res, err := (&http.Client{Timeout: 3 * time.Second}).Get("http://" + hostPort + "/healthz")
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("%s: %s", res.Status, strings.TrimSpace(string(body)))
	}
	return nil
}
