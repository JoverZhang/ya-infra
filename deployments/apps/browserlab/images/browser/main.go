// Image preparation and runtime helpers for the unchanged upstream desktop.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/JoverZhang/ya-infra/deployments/internal/host"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		return errors.New("use prepare-image, desktop, or relay")
	}
	switch args[0] {
	case "prepare-image":
		return prepareImage()
	case "desktop":
		if err := os.Chmod("/", 0755); err != nil {
			return err
		}
		return syscall.Exec("/init", append([]string{"/init"}, args[1:]...), os.Environ())
	case "relay":
		if len(args) != 3 {
			return errors.New("relay requires LISTEN TARGET")
		}
		return relay(args[1], args[2])
	default:
		return errors.New("unknown container helper action")
	}
}
func relay(listen, target string) error {
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(signals)
	go func() { <-signals; listener.Close() }()
	for {
		client, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go func() {
			defer client.Close()
			upstream, err := net.DialTimeout("tcp", target, 5*time.Second)
			if err != nil {
				log.Printf("relay dial %s: %v", target, err)
				return
			}
			defer upstream.Close()
			done := make(chan struct{})
			// Either peer can disconnect during a copy or half-close. Those errors
			// end this connection; the listener remains available for the next client.
			go func() {
				io.Copy(upstream, client)
				if tcp, ok := upstream.(*net.TCPConn); ok {
					tcp.CloseWrite()
				}
				close(done)
			}()
			io.Copy(client, upstream)
			if tcp, ok := client.(*net.TCPConn); ok {
				tcp.CloseWrite()
			}
			<-done
		}()
	}
}
func prepareImage() error {
	// Keep the existing upstream workaround; fail if its input no longer matches.
	path := "/etc/s6-overlay/s6-rc.d/svc-dbus/run"
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Contains(data, []byte("mkdir -p /run/dbus")) || !bytes.Contains(data, []byte("rm -f /run/dbus/pid")) {
		return errors.New("upstream dbus entrypoint changed; review the image workaround")
	}
	if !bytes.Contains(data, []byte("chmod 0755 /run")) {
		data = bytes.Replace(data, []byte("mkdir -p /run/dbus"), []byte("chmod 0755 /run\nmkdir -p /run/dbus"), 1)
	}
	if !bytes.Contains(data, []byte("rm -f /run/dbus/system_bus_socket")) {
		data = bytes.Replace(data, []byte("rm -f /run/dbus/pid"), []byte("rm -f /run/dbus/pid\nrm -f /run/dbus/system_bus_socket"), 1)
	}
	if err = os.WriteFile(path, data, 0755); err != nil {
		return err
	}
	// Let the upstream supervisor restart the loopback CDP relay with Chromium.
	if err := os.MkdirAll("/etc/services.d/browserlab-cdp", 0755); err != nil {
		return err
	}
	if err := os.WriteFile("/etc/services.d/browserlab-cdp/run", []byte("#!/usr/bin/execlineb -P\n/usr/local/bin/browserlab relay 0.0.0.0:9223 127.0.0.1:9222\n"), 0755); err != nil {
		return err
	}
	const keyURL = "https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key"
	response, err := (&http.Client{Timeout: 30 * time.Second}).Get(keyURL)
	if err != nil {
		return host.HTTPFailure("download NodeSource key", keyURL, err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return host.HTTPFailure("download NodeSource key", keyURL, fmt.Errorf("HTTP %d", response.StatusCode))
	}
	if err = os.MkdirAll("/etc/apt/keyrings", 0755); err != nil {
		return err
	}
	cmd := exec.Command("gpg", "--dearmor", "--yes", "-o", "/etc/apt/keyrings/nodesource.gpg")
	cmd.Stdin = io.LimitReader(response.Body, 1<<20)
	if err = cmd.Run(); err != nil {
		return err
	}
	list := "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_24.x nodistro main\n"
	if err = os.WriteFile("/etc/apt/sources.list.d/nodesource.list", []byte(list), 0644); err != nil {
		return err
	}
	version := os.Getenv("OPENCLI_VERSION")
	if version == "" || strings.ContainsAny(version, " \r\n") {
		return errors.New("OPENCLI_VERSION is required")
	}
	for _, argv := range [][]string{{"apt-get", "update"}, {"apt-get", "install", "-y", "--no-install-recommends", "nodejs=" + os.Getenv("NODE_VERSION")}, {"npm", "install", "-g", "@jackwener/opencli@" + version}, {"npm", "cache", "clean", "--force"}, {"apt-get", "clean"}} {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err = cmd.Run(); err != nil {
			return err
		}
	}
	return os.RemoveAll("/var/lib/apt/lists")
}
