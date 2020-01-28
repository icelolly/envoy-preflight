package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"time"

	"github.com/cenk/backoff"
	"github.com/icelolly/go-errors"
)

// ServerInfo is the response payload from the /server_info endpoint.
type ServerInfo struct {
	State ProxyState `json:"state"`
}

// ProxyState enumerates the state of the proxy.
type ProxyState string

// ProxyStateLive means that the proxy is in a live state.
const ProxyStateLive ProxyState = "LIVE"

func main() {
	if len(os.Args) < 2 {
		return
	}

	binary, err := exec.LookPath(os.Args[1])
	if err != nil {
		log.Fatalf("failed to find binary: %v", err)
	}

	var proc *os.Process

	// Pass signals to the child process
	go func() {
		stop := make(chan os.Signal, 2)
		signal.Notify(stop)
		for sig := range stop {
			if proc != nil {
				proc.Signal(sig)
			} else {
				// Signal received before the process even started. Let's just exit.
				os.Exit(1)
			}
		}
	}()

	active := os.Getenv("ISTIO_PROXY") == "true"
	if active {
		waitForProxy()
	}

	proc, err = os.StartProcess(binary, os.Args[1:], &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err != nil {
		log.Fatalf("error starting child process: %v", err)
	}

	state, err := proc.Wait()
	if err != nil {
		log.Fatalf("error waiting for process to finishe: %v", err)
	}

	exitCode := state.ExitCode()

	switch {
	case !active:
		// Wrapper is inactive, do nothing.
	case exitCode == 0:
		// The wrapped application had a clean exit. Kill the istio proxy.
		if err := killProxy(); err != nil {
			log.Fatalf("error killing proxy: %v", err)
		}
	}

	os.Exit(exitCode)
}

const proxyAddr = "http://127.0.0.1:15000"

func waitForProxy() {
	b := backoff.NewExponentialBackOff()

	// We wait forever for the proxy to start. In practice k8s will kill the pod if we take too long.
	b.MaxElapsedTime = 0

	_ = backoff.Retry(func() error {
		si, err := serverInfo()
		if err != nil {
			return errors.Wrap(err, "getting server info")
		}

		if si.State != ProxyStateLive {
			return errors.New("proxy not live yet")
		}

		return nil
	}, b)
}

var httpClient = &http.Client{
	Timeout: 5 * time.Second,
}

// serverInfo attempts to get the server info from the proxy's /server_info endpoint.
func serverInfo() (si *ServerInfo, err error) {
	url := fmt.Sprintf("%s/server_info", proxyAddr)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "creating request")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "requesting server info")
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil && err == nil {
			err = errors.Wrap(cerr, "closing response body")
		}
	}()

	if err := json.NewDecoder(resp.Body).Decode(&si); err != nil {
		return nil, errors.Wrap(err, "decoding server response")
	}

	return si, nil
}

// killProxy attempts to kill the proxy via the /quitquitquit endpoint.
func killProxy() (err error) {
	url := fmt.Sprintf("%s/quitquitquit", proxyAddr)

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return errors.Wrap(err, "creating request")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "sending quitquitquit")
	}

	if err := resp.Body.Close(); err != nil {
		return errors.Wrap(err, "closing response body")
	}

	return nil
}
