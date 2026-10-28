package host

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// PrepareSecret preserves an unchanged owned secret and refuses an unowned name.
// Callers hold their operation lock. Containers need recreation after a change.
func PrepareSecret(name, label, owner string, data []byte) (bool, error) {
	rootless, err := exec.Command("podman", "info", "--format", "{{.Host.Security.Rootless}}").Output()
	if err != nil {
		return false, fmt.Errorf("inspect Podman runtime: %w", err)
	}
	if strings.TrimSpace(string(rootless)) != "true" {
		return false, errors.New("secret preparation requires rootless Podman")
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	exists := exec.Command("podman", "secret", "exists", name).Run()
	if exists != nil {
		var exit *exec.ExitError
		if !errors.As(exists, &exit) || exit.ExitCode() != 1 {
			return false, fmt.Errorf("check Podman secret: %w", exists)
		}
	} else {
		data, err := exec.Command("podman", "secret", "inspect", name).Output()
		if err != nil {
			return false, fmt.Errorf("inspect Podman secret: %w", err)
		}
		var records []struct {
			Spec struct{ Labels map[string]string }
		}
		if err := json.Unmarshal(data, &records); err != nil {
			return false, fmt.Errorf("decode Podman secret metadata: %w", err)
		}
		if len(records) != 1 || records[0].Spec.Labels[label] != owner {
			return false, fmt.Errorf("Podman secret %q is not owned by %s=%s; inspect it before adoption", name, label, owner)
		}
		if records[0].Spec.Labels["ya-infra.sha256"] == digest {
			return false, nil
		}
	}
	args := []string{"secret", "create", "--label", label + "=" + owner, "--label", "ya-infra.sha256=" + digest}
	if exists == nil {
		args = append(args, "--replace")
	}
	cmd := exec.Command("podman", append(args, name, "-")...)
	cmd.Stdin = bytes.NewReader(data)
	// Keep provider output private. Secret content goes only through stdin.
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("write Podman secret: %w", err)
	}
	return true, nil
}
