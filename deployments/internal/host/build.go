package host

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

func BuildHelper(source, pkg, output string) error {
	cmd := exec.Command("go", "-C", source, "build", "-trimpath", "-o", output, pkg)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+runtime.GOARCH)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
func BuildImage(context, tag string) error {
	if err := Run("podman", "build", "--pull=missing", "--tag", tag, context); err != nil {
		return err
	}
	return Run("podman", "image", "inspect", "--format", "{{.Digest}}", tag)
}
func CopyFile(source, target string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	return os.WriteFile(target, data, info.Mode().Perm())
}
func Probe(url string, status int) error {
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Get(url)
	if err != nil {
		return HTTPFailure("probe", url, err)
	}
	defer response.Body.Close()
	if response.StatusCode != status {
		return HTTPFailure("probe", url, fmt.Errorf("HTTP status %d; expected %d", response.StatusCode, status))
	}
	return nil
}

// HTTPFailure retains transport causes without printing URL credentials or queries.
func HTTPFailure(operation, address string, err error) error {
	endpoint, parseErr := url.Parse(address)
	if parseErr != nil {
		return fmt.Errorf("%s: invalid HTTP URL", operation)
	}
	endpoint.User = nil
	endpoint.RawQuery = ""
	endpoint.ForceQuery = false
	endpoint.Fragment = ""
	var requestError *url.Error
	if errors.As(err, &requestError) {
		err = requestError.Err
	}
	return fmt.Errorf("%s %s: %w", operation, endpoint.String(), err)
}
