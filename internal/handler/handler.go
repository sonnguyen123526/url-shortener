// Package handler contains the HTTP API.
package handler

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sonnguyen/url-shortener/internal/store"
)

const (
	codeLength  = 7
	alphabet    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	maxAttempts = 5
	maxBodySize = 4 << 10 // 4 KiB is plenty for {"url": "..."}
)

type Handler struct {
	store   store.Store
	baseURL string
}

func New(s store.Store, baseURL string) *Handler {
	return &Handler{store: s, baseURL: strings.TrimRight(baseURL, "/")}
}

// Routes uses Go 1.22+ pattern routing: "METHOD /path/{wildcard}".
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /shorten", h.shorten)
	mux.HandleFunc("GET /healthz", h.healthz) // liveness: is the process alive?
	mux.HandleFunc("GET /readyz", h.readyz)   // readiness: can we serve traffic (Redis reachable)?
	mux.HandleFunc("GET /{code}", h.redirect)
	return mux
}

type shortenRequest struct {
	URL string `json:"url"`
}

type shortenResponse struct {
	Code     string `json:"code"`
	ShortURL string `json:"short_url"`
}

func (h *Handler) shorten(w http.ResponseWriter, r *http.Request) {
	var req shortenRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodySize)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if !isValidURL(req.URL) {
		writeError(w, http.StatusBadRequest, "url must be an absolute http(s) URL")
		return
	}

	// Random codes can collide; retry a few times before giving up.
	for range maxAttempts {
		code, err := randomCode(codeLength)
		if err != nil {
			slog.Error("generate code", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		saved, err := h.store.Save(r.Context(), code, req.URL)
		if err != nil {
			slog.Error("save url", "err", err)
			writeError(w, http.StatusServiceUnavailable, "storage unavailable")
			return
		}
		if saved {
			slog.Info("shortened", "code", code, "url", req.URL)
			writeJSON(w, http.StatusCreated, shortenResponse{Code: code, ShortURL: h.baseURL + "/" + code})
			return
		}
	}
	writeError(w, http.StatusInternalServerError, "could not allocate a unique code")
}

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	target, err := h.store.Get(r.Context(), code)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "short code not found")
		return
	}
	if err != nil {
		slog.Error("get url", "code", code, "err", err)
		writeError(w, http.StatusServiceUnavailable, "storage unavailable")
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.store.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "storage not ready")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func isValidURL(raw string) bool {
	u, err := url.ParseRequestURI(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func randomCode(n int) (string, error) {
	b := make([]byte, n)
	max := big.NewInt(int64(len(alphabet)))
	for i := range b {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = alphabet[idx.Int64()]
	}
	return string(b), nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
