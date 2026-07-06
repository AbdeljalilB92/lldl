package app

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/AbdeljalilB92/lldl/shared/logging"
	"github.com/AbdeljalilB92/lldl/shared/validation"
)

const tokenServerAddr = "127.0.0.1:38479"

const tokenServerShutdownTimeout = 5 * time.Second

// tokenRequest is the JSON body expected by POST /token.
type tokenRequest struct {
	Token string `json:"token"`
}

// TokenServer listens on a local port and receives auth tokens via HTTP.
// External tools (e.g. browser extensions) POST the token here so the GUI
// can pick it up without manual copy-paste.
type TokenServer struct {
	server   *http.Server
	listener net.Listener

	// emitTokenEvent is called after a valid token is received.
	// In production this wraps WailsService.emitEvent("token:received", token).
	emitTokenEvent func(token string)
}

// NewTokenServer creates a TokenServer that calls emitTokenEvent when a valid
// token is received. The emitTokenEvent callback decouples the server from
// Wails runtime details, making it testable without a Wails context.
func NewTokenServer(emitTokenEvent func(token string)) *TokenServer {
	ts := &TokenServer{
		emitTokenEvent: emitTokenEvent,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", ts.handlePostToken)
	mux.HandleFunc("GET /health", ts.handleHealth)

	ts.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	return ts
}

// Start binds to the configured address and begins serving requests in the
// background. Returns as soon as the listener is ready so callers don't have
// to race against the server goroutine.
func (ts *TokenServer) Start(ctx context.Context) error {
	logger := logging.New("[TokenServer][Start]")

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", tokenServerAddr)
	if err != nil {
		return err
	}
	ts.listener = ln

	go func() {
		if err := ts.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.Warn("server exited unexpectedly", "error", err)
		}
	}()

	logger.Info("listening", "addr", tokenServerAddr)
	return nil
}

// Stop performs a graceful shutdown. The server is given 5 seconds to finish
// in-flight requests before the underlying listener is closed.
func (ts *TokenServer) Stop() {
	logger := logging.New("[TokenServer][Stop]")

	ctx, cancel := context.WithTimeout(context.Background(), tokenServerShutdownTimeout)
	defer cancel()

	if err := ts.server.Shutdown(ctx); err != nil {
		logger.Warn("shutdown error", "error", err)
	}
}

func (ts *TokenServer) handlePostToken(w http.ResponseWriter, r *http.Request) {
	var req tokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if err := validation.ValidateToken(req.Token); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if ts.emitTokenEvent != nil {
		ts.emitTokenEvent(req.Token)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (ts *TokenServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
