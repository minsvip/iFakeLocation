package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"ifakelocation/internal/location"
	"ifakelocation/internal/server"
	"ifakelocation/resources"
)

var appVersion = "1.0.0"

func tryBindFreePort() (net.Listener, int, error) {
	const minPort = 49215
	const maxPort = 65535

	for port := minPort; port < maxPort; port++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			return l, port, nil
		}
	}

	// Fallback to dynamic system port
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	return l, port, nil
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func main() {
	listener, port, err := tryBindFreePort()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialise iFakeLocation (no free ports on local system): %v\n", err)
		os.Exit(1)
	}

	appServer := server.NewServer(resources.FS, appVersion)
	url := fmt.Sprintf("http://localhost:%d/", port)

	// Open browser
	if err := openBrowser(url); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to start default web browser: %v\n", err)
	}

	fmt.Println("==================================================")
	fmt.Printf("  iFakeLocation (Go) v%s is now running\n", appVersion)
	fmt.Printf("  URL: %s\n", url)
	fmt.Println("==================================================")
	fmt.Println("\nPress Ctrl-C to quit (or click the close button in the web UI).")

	httpServer := &http.Server{
		Handler:      appServer.Handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	// Graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutting down iFakeLocation...")
		location.CloseAllSessions()
		_ = httpServer.Close()
		os.Exit(0)
	}()

	if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
	}
}
