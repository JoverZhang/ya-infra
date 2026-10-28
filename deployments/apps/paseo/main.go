// Paseo host hooks keep build artifacts separate from runtime credentials.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/JoverZhang/ya-infra/deployments/internal/host"
)

const paseoVersion = "0.9.1"
const codexVersion = "0.159.2"
const nodeVersion = "25.9.0"

type artifactVersions struct {
	Paseo string `json:"paseo_version"`
	Codex string `json:"codex_version"`
	Node  string `json:"node_version"`
}

type artifactManifest struct {
	Versions artifactVersions  `json:"versions"`
	SHA256   map[string]string `json:"sha256"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 || args[0] == "--help" {
		fmt.Println("Usage: paseo build [--artifacts DIR] | verify --web-url URL --daemon-url URL\nBuild creates local images only. Verify performs read-only HTTP checks.")
		return nil
	}
	switch args[0] {
	case "build":
		flags := flag.NewFlagSet("build", flag.ContinueOnError)
		artifacts := flags.String("artifacts", filepath.Join(host.XDG("XDG_CACHE_HOME", ".cache"), "infra", "paseo", paseoVersion+"-"+codexVersion+"-"+nodeVersion+"-bundle2"), "verified public program snapshot")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("build takes no positional arguments")
		}
		if err := prepareArtifacts(*artifacts); err != nil {
			return err
		}
		temp, err := os.MkdirTemp("", "ya-infra-paseo-build-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(temp)
		for _, image := range []string{"daemon", "web"} {
			context := filepath.Join(temp, image)
			if err = os.Mkdir(context, 0700); err != nil {
				return err
			}
			base := filepath.Join("images", image)
			if err = host.CopyFile(filepath.Join(base, "Containerfile"), filepath.Join(context, "Containerfile")); err != nil {
				return err
			}
			if image == "daemon" {
				if err = copyPublic(*artifacts, filepath.Join(context, "artifacts")); err != nil {
					return err
				}
				if err = host.CopyFile(filepath.Join(base, "codex.toml"), filepath.Join(context, "codex.toml")); err != nil {
					return err
				}
				if err = host.BuildHelper("../..", "./apps/paseo/images/daemon", filepath.Join(context, "initialize")); err != nil {
					return err
				}
			} else {
				if err = copyPublic(filepath.Join(*artifacts, "paseo/node_modules/@getpaseo/server/dist/server/web-ui"), filepath.Join(context, "web-ui")); err != nil {
					return err
				}
				if err = host.CopyFile(filepath.Join(base, "nginx.conf"), filepath.Join(context, "nginx.conf")); err != nil {
					return err
				}
			}
			if err = host.BuildImage(context, "localhost/ya-infra-paseo-"+image+":"+paseoVersion); err != nil {
				return err
			}
		}
		return nil
	case "verify":
		flags := flag.NewFlagSet("verify", flag.ContinueOnError)
		web := flags.String("web-url", "", "forwarded Web origin")
		daemon := flags.String("daemon-url", "", "forwarded daemon origin")
		secretFile := flags.String("secret-file", "config.sops.env", "encrypted Compose environment")
		passwordKey := flags.String("password-key", "PASEO_PASSWORD", "environment key for the selected daemon password")
		hostnamesKey := flags.String("hostnames-key", "PASEO_HOSTNAMES", "environment key for the selected daemon hostnames")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 || *web == "" || *daemon == "" {
			return errors.New("verify requires --web-url and --daemon-url")
		}
		if err := host.Probe(strings.TrimRight(*web, "/")+"/healthz", 200); err != nil {
			return err
		}
		var values map[string]string
		if err := host.Decrypt(*secretFile, &values); err != nil {
			return err
		}
		password, names := values[*passwordKey], values[*hostnamesKey]
		if password == "" {
			return errors.New("encrypted environment has no selected daemon password")
		}
		endpoint, err := url.Parse(*daemon)
		if err != nil || endpoint.User != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.RawQuery != "" || endpoint.Fragment != "" || (endpoint.Path != "" && endpoint.Path != "/") {
			return errors.New("daemon-url must be an HTTP(S) origin")
		}
		allowed := endpoint.Hostname() == "localhost"
		if ip := net.ParseIP(endpoint.Hostname()); ip != nil && ip.IsLoopback() {
			allowed = true
		}
		for _, name := range strings.Split(names, ",") {
			if name == endpoint.Hostname() {
				allowed = true
			}
		}
		if !allowed {
			return errors.New("daemon-url hostname is not declared in the encrypted environment")
		}
		if err = verifyAPI(strings.TrimRight(*daemon, "/"), password); err != nil {
			return err
		}
		port := endpoint.Port()
		if port == "" {
			port = "80"
			if endpoint.Scheme == "https" {
				port = "443"
			}
		}
		connection := "tcp://" + net.JoinHostPort(endpoint.Hostname(), port)
		if endpoint.Scheme == "https" {
			connection += "?ssl=true"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// The native CLI creates a client ID even for ls. Confine that state to a
		// temporary home so verification cannot initialize the operator's profile.
		clientHome, err := os.MkdirTemp("", "ya-infra-paseo-verify-")
		if err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, "paseo", "ls", "--host", connection, "--json")
		// Paseo 0.9.1 resolves the client ID from HOME even with PASEO_HOME set.
		cmd.Env = append(os.Environ(), "PASEO_PASSWORD="+password, "HOME="+clientHome, "PASEO_HOME="+filepath.Join(clientHome, ".paseo"))
		probeErr := cmd.Run()
		cleanupErr := os.RemoveAll(clientHome)
		if probeErr != nil {
			probeErr = fmt.Errorf("authenticated WebSocket check failed: %w", probeErr)
		}
		if cleanupErr != nil {
			cleanupErr = fmt.Errorf("remove temporary Paseo client state %q: %w", clientHome, cleanupErr)
		}
		return errors.Join(probeErr, cleanupErr)
	default:
		return errors.New("unsupported action; use --help")
	}
}
func privateName(name string) bool {
	return name == "auth.json" || name == ".npmrc" || strings.HasPrefix(name, ".env")
}
func copyPublic(source, target string) error {
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, item fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if privateName(item.Name()) {
			if item.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if item.Type()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(source, resolved)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				return errors.New("public artifact has an external symlink")
			}
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if filepath.IsAbs(link) {
				link, err = filepath.Rel(filepath.Dir(path), resolved)
				if err != nil {
					return err
				}
			}
			return os.Symlink(link, destination)
		}
		if item.IsDir() {
			return os.MkdirAll(destination, 0755)
		}
		if !item.Type().IsRegular() {
			return errors.New("public artifact is not a regular file")
		}
		return host.CopyFile(path, destination)
	})
}
func hashes(root string) (map[string]string, error) {
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, item fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if item.IsDir() || path == filepath.Join(root, "manifest.json") {
			return nil
		}
		if privateName(item.Name()) {
			return errors.New("snapshot contains a credential filename")
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, resolved)
		if err != nil || relative == ".." || strings.HasPrefix(relative, "../") {
			return errors.New("snapshot contains an external symlink")
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		sum := sha256.New()
		if _, err = io.Copy(sum, f); err != nil {
			return err
		}
		key, _ := filepath.Rel(root, path)
		result[key] = hex.EncodeToString(sum.Sum(nil))
		return nil
	})
	return result, err
}
func prepareArtifacts(target string) error {
	target, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	manifest := filepath.Join(target, "manifest.json")
	if data, err := os.ReadFile(manifest); err == nil {
		var saved artifactManifest
		if err := json.Unmarshal(data, &saved); err != nil {
			return fmt.Errorf("decode public snapshot manifest %q: %w", manifest, err)
		}
		if saved.Versions != (artifactVersions{Paseo: paseoVersion, Codex: codexVersion, Node: nodeVersion}) {
			return errors.New("snapshot version differs from the application pins")
		}
		actual, err := hashes(target)
		if err != nil {
			return err
		}
		if len(actual) != len(saved.SHA256) {
			return errors.New("snapshot file set differs from its manifest")
		}
		for key, value := range actual {
			if saved.SHA256[key] != value {
				return errors.New("snapshot checksum mismatch")
			}
		}
		return nil
	}
	if _, err := os.Stat(target); err == nil {
		return errors.New("artifact directory exists without a valid manifest")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		return err
	}
	node, err = filepath.EvalSymlinks(node)
	if err != nil {
		return err
	}
	paseo, err := exec.LookPath("paseo")
	if err != nil {
		return err
	}
	paseo, err = filepath.EvalSymlinks(paseo)
	if err != nil {
		return err
	}
	paseo = filepath.Dir(filepath.Dir(paseo))
	codex, err := exec.LookPath("codex")
	if err != nil {
		return err
	}
	codex, err = filepath.EvalSymlinks(codex)
	if err != nil {
		return err
	}
	for _, check := range []struct{ path, want string }{{node, "v" + nodeVersion}, {codex, "codex-cli " + codexVersion}} {
		out, err := exec.Command(check.path, "--version").Output()
		if err != nil || strings.TrimSpace(string(out)) != check.want {
			return errors.New("installed public program version differs from the application pins")
		}
	}
	var pkg struct{ Version string }
	data, err := os.ReadFile(filepath.Join(paseo, "package.json"))
	if err != nil {
		return err
	}
	if json.Unmarshal(data, &pkg) != nil || pkg.Version != paseoVersion {
		return errors.New("installed Paseo version differs from the application pin")
	}
	dist := filepath.Dir(filepath.Dir(codex))
	for _, name := range []string{"bin/codex-code-mode-host", "codex-package.json"} {
		if _, err = os.Stat(filepath.Join(dist, name)); err != nil {
			return errors.New("standalone Codex distribution is incomplete")
		}
	}
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(filepath.Dir(target), ".snapshot-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	if err = host.CopyFile(node, filepath.Join(temp, "node")); err != nil {
		return err
	}
	if err = copyPublic(paseo, filepath.Join(temp, "paseo")); err != nil {
		return err
	}
	if err = copyPublic(dist, filepath.Join(temp, "codex-dist")); err != nil {
		return err
	}
	sums, err := hashes(temp)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(artifactManifest{
		Versions: artifactVersions{Paseo: paseoVersion, Codex: codexVersion, Node: nodeVersion},
		SHA256:   sums,
	})
	if err != nil {
		return err
	}
	if err = host.WritePrivate(filepath.Join(temp, "manifest.json"), encoded); err != nil {
		return err
	}
	return os.Rename(temp, target)
}

func verifyAPI(origin, password string) error {
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	wrong := hex.EncodeToString(random[:])
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, probe := range []struct {
		credential string
		status     int
	}{{"", 401}, {wrong, 401}, {password, 200}} {
		request, err := http.NewRequest("GET", origin+"/api/status", nil)
		if err != nil {
			return errors.New("invalid daemon origin")
		}
		if probe.credential != "" {
			request.Header.Set("Authorization", "Bearer "+probe.credential)
		}
		response, err := client.Do(request)
		if err != nil {
			return host.HTTPFailure("daemon authentication probe", request.URL.String(), err)
		}
		response.Body.Close()
		if response.StatusCode != probe.status {
			return fmt.Errorf("daemon authentication status %d; expected %d", response.StatusCode, probe.status)
		}
	}
	return nil
}
