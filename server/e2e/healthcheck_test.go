//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestHealthcheck(t *testing.T) {
	s := startServer(t, newDB(t), time.Now().UnixMilli())
	res, err := http.Get(s.url + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || string(body) != "ok" || res.Header.Get("Cache-Control") != "no-store" ||
		res.Header.Get("Content-Security-Policy") == "" {
		t.Fatalf("/healthz: %d %q %v", res.StatusCode, body, res.Header)
	}

	check := func(port int) ([]byte, error) {
		cmd := exec.Command(binary, "healthcheck")
		cmd.Env = append(os.Environ(), fmt.Sprintf("LH_HTTP_ADDR=127.0.0.1:%d", port))
		return cmd.CombinedOutput()
	}
	if out, err := check(s.port); err != nil {
		t.Fatalf("against the running server: %v\n%s", err, out)
	}
	var exit *exec.ExitError
	if out, err := check(freePort(t)); !errors.As(err, &exit) || exit.ExitCode() != 1 || len(out) == 0 {
		t.Fatalf("against a port nothing listens on: %v\n%s", err, out)
	}
}
