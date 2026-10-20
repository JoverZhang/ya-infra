package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/JoverZhang/ya-infra/deployments/internal/host"
	"gopkg.in/yaml.v3"
)

const serverVersion = "v1.37.1+k3s1"
const kubectlVersion = "v1.37.1"

var clientTools = []struct{ name, module, version string }{
	{"sops", "github.com/getsops/sops/v3/cmd/sops", "v3.13.3"},
	{"age", "filippo.io/age/cmd/age", "v1.2.1"},
	{"age-keygen", "filippo.io/age/cmd/age-keygen", "v1.2.1"},
	{"kustomize", "sigs.k8s.io/kustomize/kustomize/v5", "v5.8.1"},
}

func fetch(url string, limit int64) ([]byte, error) {
	c := &http.Client{Timeout: 2 * time.Minute}
	r, err := c.Get(url)
	if err != nil {
		return nil, host.HTTPFailure("download", url, err)
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return nil, host.HTTPFailure("download", url, fmt.Errorf("HTTP %d", r.StatusCode))
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if int64(len(data)) > limit {
		return nil, host.HTTPFailure("download", url, errors.New("response exceeds size limit"))
	}
	if err != nil {
		return nil, host.HTTPFailure("read download", url, err)
	}
	return data, nil
}
func installClientTools() error {
	dir := filepath.Join(host.XDG("XDG_DATA_HOME", ".local/share"), "ya-infra", "bin")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	for _, tool := range clientTools {
		binary := filepath.Join(dir, tool.name)
		info, err := buildinfo.ReadFile(binary)
		if err == nil && info.Main.Version == tool.version {
			continue
		}
		fmt.Println("Installing", tool.name, tool.version)
		cmd := exec.Command("go", "install", tool.module+"@"+tool.version)
		cmd.Env = append(os.Environ(), "GOBIN="+dir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err = cmd.Run(); err != nil {
			return err
		}
	}
	if err := installKSOPS(dir); err != nil {
		return err
	}
	path := filepath.Join(dir, "kubectl")
	out, _ := exec.Command(path, "version", "--client", "-o", "json").Output()
	if !bytes.Contains(out, []byte(`"gitVersion": "`+kubectlVersion+`"`)) {
		if runtime.GOOS != "linux" || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") {
			return errors.New("automatic kubectl installation supports Linux amd64 and arm64")
		}
		url := "https://dl.k8s.io/release/" + kubectlVersion + "/bin/" + runtime.GOOS + "/" + runtime.GOARCH + "/kubectl"
		checksum, err := fetch(url+".sha256", 1024)
		if err != nil {
			return err
		}
		data, err := fetch(url, 150<<20)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != strings.TrimSpace(string(checksum)) {
			return errors.New("kubectl checksum mismatch")
		}
		if err = host.WritePrivate(path, data); err != nil {
			return err
		}
		if err = os.Chmod(path, 0700); err != nil {
			return err
		}
	}
	os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	fmt.Println("Client tools:", dir)
	return nil
}

func localConfiguration(config map[string]any, previousRoot string) ([]byte, error) {
	root := filepath.Join(host.XDG("XDG_DATA_HOME", ".local/share"), "ya-infra", "pv")
	if previousRoot != "" && previousRoot != root {
		return nil, errors.New("data root changed; migrate persistent data before changing this path")
	}
	if specified, ok := config["default-local-storage-path"]; ok && specified != root {
		return nil, errors.New("default-local-storage-path must match the invoking user's XDG data root")
	}
	if value, ok := config["secrets-encryption"]; ok && value != true {
		return nil, errors.New("local server requires secrets-encryption: true")
	}
	config["default-local-storage-path"] = root
	config["secrets-encryption"] = true
	config["write-kubeconfig-mode"] = "0600"
	return yaml.Marshal(config)
}

func bootstrap(args []string) error {
	flags := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	local := flags.Bool("local", false, "install or reconcile the local server using sudo")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("bootstrap takes no positional arguments")
	}
	if err := installClientTools(); err != nil {
		return err
	}
	if !*local {
		return nil
	}
	if os.Geteuid() == 0 {
		return errors.New("run bootstrap as the ordinary user; it calls sudo only for system changes")
	}
	config, err := host.Load()
	if err != nil {
		return err
	}
	return bootstrapLocal(config)
}

func bootstrapLocal(config host.Config) error {
	destination := os.Getenv("KUBECONFIG")
	if destination == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("locate kubeconfig destination: %w", err)
		}
		destination = filepath.Join(home, ".kube", "config")
	}
	if strings.ContainsRune(destination, os.PathListSeparator) {
		return errors.New("bootstrap needs one KUBECONFIG destination; kubectl can still use multiple files")
	}
	existing := &kubeconfig{}
	old, err := os.ReadFile(destination)
	if err == nil {
		existing, err = decodeKubeconfig(old)
		if err != nil {
			return fmt.Errorf("invalid existing kubeconfig %q: %w", destination, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	var options map[string]any
	if err := host.Decrypt(filepath.Join(config.Source, "kubernetes", "bootstrap", "local.sops.yaml"), &options); err != nil {
		return err
	}
	if options == nil {
		return errors.New("local server configuration must be a YAML mapping")
	}
	const target = "/etc/rancher/k3s/ya-infra.yaml"
	managed := exec.Command("sudo", "test", "-f", target).Run() == nil
	_, binaryErr := exec.LookPath("k3s")
	binaryPresent := binaryErr == nil
	var before []byte
	previousRoot := ""
	if managed {
		before, err = exec.Command("sudo", "cat", target).Output()
		if err != nil {
			return errors.New("cannot read the installed server configuration")
		}
		var old map[string]any
		if yaml.Unmarshal(before, &old) != nil {
			return errors.New("invalid installed server configuration")
		}
		previousRoot, _ = old["default-local-storage-path"].(string)
		if previousRoot == "" {
			return errors.New("installed server configuration has no recorded data root")
		}
	} else if binaryPresent {
		return errors.New("existing local server is not owned by this bootstrap; inspect it before adoption")
	}
	if !binaryPresent {
		if err = preflightLocal(options); err != nil {
			return err
		}
	}
	data, err := localConfiguration(options, previousRoot)
	if err != nil {
		return err
	}
	defer clear(data)
	serviceState, err := exec.Command("systemctl", "show", "k3s.service", "--property=LoadState", "--value").Output()
	if err != nil {
		return fmt.Errorf("inspect local server service: %w", err)
	}
	state := strings.TrimSpace(string(serviceState))
	if state != "loaded" && state != "not-found" {
		return fmt.Errorf("local server service has unexpected load state %q; inspect systemctl status k3s", state)
	}
	installed := binaryPresent && state == "loaded"
	changed := !installed || !bytes.Equal(before, data)
	const pendingRestart = target + ".restart-pending"
	pending := exec.Command("sudo", "test", "-f", pendingRestart).Run() == nil
	if binaryPresent {
		out, err := exec.Command("sudo", "k3s", "--version").Output()
		if err != nil || !bytes.Contains(out, []byte(serverVersion)) {
			return errors.New("local server version differs; use the documented backup and upgrade procedure")
		}
	}
	if changed || pending {
		temp, err := os.MkdirTemp("", "ya-infra-bootstrap-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(temp)
		input := filepath.Join(temp, "server.yaml")
		if err = host.WritePrivate(input, data); err != nil {
			return err
		}
		if err = host.Run("sudo", "install", "-d", "-m", "0700", "/etc/rancher/k3s"); err != nil {
			return err
		}
		// Record intent before changing the file; keep it until the service has restarted.
		// A retry must reconcile the running service even when the file already matches.
		if err = host.Run("sudo", "touch", pendingRestart); err != nil {
			return fmt.Errorf("save pending server restart: %w", err)
		}
		if err = host.Run("sudo", "install", "-m", "0600", input, target); err != nil {
			return err
		}
		if installed {
			if err = host.Run("sudo", "systemctl", "restart", "k3s"); err != nil {
				return err
			}
		} else {
			// A binary alone does not complete installation. Resume the pinned installer
			// when an interrupted owned installation has not created its service.
			// The upstream installer verifies the server binary checksum.
			installer, err := fetch("https://raw.githubusercontent.com/k3s-io/k3s/"+serverVersion+"/install.sh", 1<<20)
			if err != nil {
				return err
			}
			path := filepath.Join(temp, "install.sh")
			if err = host.WritePrivate(path, installer); err != nil {
				return err
			}
			if err = host.Run("sudo", "env", "INSTALL_K3S_VERSION="+serverVersion, "INSTALL_K3S_EXEC=server --config "+target, "INSTALL_K3S_SYMLINK=skip", "sh", path); err != nil {
				return err
			}
		}
		if err = host.Run("sudo", "rm", pendingRestart); err != nil {
			return fmt.Errorf("clear completed server restart: %w", err)
		}
	} else if exec.Command("systemctl", "is-active", "--quiet", "k3s").Run() != nil {
		if err = host.Run("sudo", "systemctl", "start", "k3s"); err != nil {
			return err
		}
	}
	raw, err := exec.Command("sudo", "cat", "/etc/rancher/k3s/k3s.yaml").Output()
	if err != nil {
		return errors.New("local kubeconfig is not ready; inspect systemctl status k3s and retry bootstrap")
	}
	defer clear(raw)
	localConfig, err := decodeKubeconfig(raw)
	if err != nil {
		return fmt.Errorf("invalid local kubeconfig: %w", err)
	}
	if err = mergeKubeconfig(existing, localConfig, config.Context); err != nil {
		return err
	}
	merged, err := yaml.Marshal(existing)
	if err != nil {
		return err
	}
	defer clear(merged)
	if err = host.WritePrivate(destination, merged); err != nil {
		return err
	}
	fmt.Println("Local context ready:", config.Context)
	return host.Run("kubectl", "--context="+config.Context, "wait", "--for=condition=Ready", "nodes", "--all", "--timeout=120s")
}

func installKSOPS(dir string) error {
	const version = "4.5.1"
	binary := filepath.Join(dir, "ksops")
	stamp := binary + ".sha256"
	if data, err := os.ReadFile(binary); err == nil {
		sum := sha256.Sum256(data)
		saved, _ := os.ReadFile(stamp)
		if string(saved) == version+" "+hex.EncodeToString(sum[:]) {
			return nil
		}
	}
	arch, expected := "", ""
	switch runtime.GOARCH {
	case "amd64":
		arch, expected = "x86_64", "9641e03a301bf2fc4b98d0e837a0351e1ca443ecd7a68b25e6af2149fbb2ef30"
	case "arm64":
		arch, expected = "arm64", "c7bb562cc2d580a1decc6c77a35e4d35575fb01ca003425726004ecd13dc1cb5"
	default:
		return errors.New("KSOPS installation supports Linux amd64 and arm64")
	}
	if runtime.GOOS != "linux" {
		return errors.New("bootstrap supports Linux")
	}
	data, err := fetch("https://github.com/viaduct-ai/kustomize-sops/releases/download/v"+version+"/ksops_"+version+"_Linux_"+arch+".tar.gz", 100<<20)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != expected {
		return errors.New("KSOPS archive checksum mismatch")
	}
	archive, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer archive.Close()
	reader := tar.NewReader(archive)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Name != "ksops" || header.Typeflag != tar.TypeReg {
			continue
		}
		content, err := io.ReadAll(io.LimitReader(reader, 200<<20))
		if err != nil {
			return err
		}
		if err = host.WritePrivate(binary, content); err != nil {
			return err
		}
		if err = os.Chmod(binary, 0700); err != nil {
			return err
		}
		sum := sha256.Sum256(content)
		return host.WritePrivate(stamp, []byte(version+" "+hex.EncodeToString(sum[:])))
	}
	return errors.New("KSOPS archive has no executable")
}

// Check the native Pod and Service CIDRs against existing host routes before installation.
func checkRanges(options map[string]any, occupied []netip.Prefix) error {
	var selected []netip.Prefix
	for _, entry := range []struct{ name, fallback string }{{"cluster-cidr", "10.42.0.0/16"}, {"service-cidr", "10.43.0.0/16"}} {
		value := entry.fallback
		if configured, exists := options[entry.name]; exists {
			switch typed := configured.(type) {
			case string:
				value = typed
			case []any:
				parts := []string{}
				for _, item := range typed {
					text, ok := item.(string)
					if !ok {
						return fmt.Errorf("invalid %s", entry.name)
					}
					parts = append(parts, text)
				}
				value = strings.Join(parts, ",")
			default:
				return fmt.Errorf("invalid %s", entry.name)
			}
		}
		for _, cidr := range strings.Split(value, ",") {
			prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
			if err != nil {
				return fmt.Errorf("invalid %s network", entry.name)
			}
			for _, other := range append(append([]netip.Prefix{}, occupied...), selected...) {
				if prefix.Overlaps(other) {
					return fmt.Errorf("%s overlaps a host route or another cluster range; select unused CIDRs", entry.name)
				}
			}
			selected = append(selected, prefix)
		}
	}
	return nil
}

func preflightLocal(options map[string]any) error {
	for _, unit := range []string{"k3s.service", "k3s-agent.service", "kubelet.service"} {
		out, err := exec.Command("systemctl", "show", unit, "--property=LoadState", "--value").Output()
		if err != nil {
			return errors.New("cannot inspect system services")
		}
		if strings.TrimSpace(string(out)) != "not-found" {
			return fmt.Errorf("existing %s requires explicit adoption", unit)
		}
	}
	ports := []string{"6444", "10010", "10248", "10249", "10250", "10256", "10257", "10258", "10259"}
	apiPort := "6443"
	if port, exists := options["https-listen-port"]; exists {
		apiPort = fmt.Sprint(port)
	}
	for _, port := range append(ports, apiPort) {
		listener, err := net.Listen("tcp", ":"+port)
		if err != nil {
			return fmt.Errorf("TCP port %s is unavailable", port)
		}
		listener.Close()
	}
	backend, _ := options["flannel-backend"].(string)
	udpPorts := []int{8472}
	switch backend {
	case "wireguard-native":
		udpPorts = []int{51820, 51821}
	case "none", "host-gw":
		udpPorts = nil
	}
	for _, port := range udpPorts {
		listener, err := net.ListenPacket("udp", fmt.Sprintf(":%d", port))
		if err != nil {
			return fmt.Errorf("UDP port %d is unavailable", port)
		}
		listener.Close()
	}
	var occupied []netip.Prefix
	for _, family := range []string{"-4", "-6"} {
		out, err := exec.Command("ip", family, "-j", "route", "show", "table", "all").Output()
		if err != nil {
			return errors.New("cannot inspect host network ranges; install iproute2")
		}
		var routes []struct {
			Destination string `json:"dst"`
		}
		if json.Unmarshal(out, &routes) != nil {
			return errors.New("invalid host route output")
		}
		for _, route := range routes {
			if prefix, err := netip.ParsePrefix(route.Destination); err == nil && prefix.Bits() != 0 {
				occupied = append(occupied, prefix)
			}
		}
	}
	if err := checkRanges(options, occupied); err != nil {
		return err
	}
	for _, path := range []string{"/var/lib", host.XDG("XDG_DATA_HOME", ".local/share")} {
		for {
			if _, err := os.Stat(path); err == nil {
				break
			}
			if path == filepath.Dir(path) {
				return errors.New("data filesystem is unavailable")
			}
			path = filepath.Dir(path)
		}
		var disk syscall.Statfs_t
		if err := syscall.Statfs(path, &disk); err != nil {
			return err
		}
		if disk.Bavail*uint64(disk.Bsize) < 2<<30 {
			return errors.New("bootstrap needs at least 2 GiB free on the server and application data filesystems")
		}
	}
	return nil
}
