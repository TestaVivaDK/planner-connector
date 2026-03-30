package auth

import (
	"context"
	"fmt"
	"net"
	"net/http"
)

// StartLoopbackServer starts an HTTP server on a random port on 127.0.0.1.
// It returns the port, a channel that receives the authorization code,
// an error channel, and a cleanup function to shut down the server.
func StartLoopbackServer() (port int, codeCh <-chan string, errCh <-chan error, cleanup func()) {
	code := make(chan string, 1)
	errC := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		c := r.URL.Query().Get("code")
		errParam := r.URL.Query().Get("error")

		if c != "" {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "<h1>Login successful</h1><p>You can close this window and return to Claude.</p>")
			// Non-blocking send; channel is buffered.
			select {
			case code <- c:
			default:
			}
		} else {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "<h1>Login failed</h1><p>Something went wrong. Please try again.</p>")
			msg := errParam
			if msg == "" {
				msg = "No authorization code received"
			}
			select {
			case errC <- fmt.Errorf("%s", msg):
			default:
			}
		}
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		errC <- fmt.Errorf("failed to start loopback listener: %w", err)
		return 0, code, errC, func() {}
	}

	srv := &http.Server{Handler: mux}

	go srv.Serve(listener) //nolint:errcheck

	addr := listener.Addr().(*net.TCPAddr)

	cleanup = func() {
		srv.Shutdown(context.Background()) //nolint:errcheck
	}

	return addr.Port, code, errC, cleanup
}
