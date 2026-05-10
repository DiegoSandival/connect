package portal

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"hosts/internal/config"
	"hosts/internal/network"
	"hosts/internal/sessions"
	"hosts/internal/uploads"
)

var macPattern = regexp.MustCompile(`(?i)([0-9a-f]{2}[-:]){5}[0-9a-f]{2}`)

const sessionCookieName = "portal_session"

type Server struct {
	cfg        config.Config
	tmpl       *template.Template
	static     http.Handler
	logger     *slog.Logger
	sessions   *sessions.Manager
	uploads    *uploads.Service
	controller network.AccessController
}

type pageData struct {
	BusinessName   string
	PortalBaseURL  string
	AccessMinutes  int
	ActivationMode string
	ConfigJSON     template.JS
}

type overviewResponse struct {
	Controller string                `json:"controller"`
	Mode       string                `json:"mode"`
	Sessions   []sessions.Snapshot   `json:"sessions"`
	Uploads    []uploads.SavedUpload `json:"uploads"`
}

func NewServer(cfg config.Config, logger *slog.Logger, sessionManager *sessions.Manager, uploadService *uploads.Service, controller network.AccessController) (*Server, error) {
	tmpl, err := template.ParseFS(assets, "assets/index.html")
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	staticFS, err := fs.Sub(assets, "assets")
	if err != nil {
		return nil, fmt.Errorf("static assets: %w", err)
	}

	return &Server{
		cfg:        cfg,
		tmpl:       tmpl,
		static:     http.FileServer(http.FS(staticFS)),
		logger:     logger,
		sessions:   sessionManager,
		uploads:    uploadService,
		controller: controller,
	}, nil
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/client", s.handleClient)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/animal/change", s.handleChangeAnimal)
	mux.HandleFunc("/api/access", s.handleAccess)
	mux.HandleFunc("/api/upload/delete", s.handleDeleteUpload)
	mux.HandleFunc("/api/uploads", s.handleClientUploads)
	mux.HandleFunc("/api/upload", s.handleUpload)
	mux.HandleFunc("/api/overview", s.handleOverview)
	mux.Handle("/static/", http.StripPrefix("/static/", s.static))

	return s.withLogging(mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	configJSON, err := json.Marshal(map[string]any{
		"activationMode": s.cfg.Network.ActivationMode,
		"accessMinutes":  s.cfg.Network.AccessDurationMinutes,
	})
	if err != nil {
		http.Error(w, "no se pudo cargar la pagina", http.StatusInternalServerError)
		return
	}

	data := pageData{
		BusinessName:   s.cfg.BusinessName,
		PortalBaseURL:  s.cfg.Network.PortalBaseURL,
		AccessMinutes:  s.cfg.Network.AccessDurationMinutes,
		ActivationMode: s.cfg.Network.ActivationMode,
		ConfigJSON:     template.JS(configJSON),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.Execute(w, data); err != nil {
		s.logger.Error("render index", "error", err)
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}

	client := s.clientInfo(r)
	snapshot, err := s.sessions.Touch(r.Context(), client)
	if err != nil {
		s.logger.Error("touch client", "ip", client.IP, "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error":   "no se pudo preparar el acceso temporal",
			"details": err.Error(),
		})
		return
	}

	s.writeSessionCookie(w, snapshot.ID)
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}

	client := s.clientInfo(r)
	snapshot, err := s.sessions.Ensure(client)
	if err != nil {
		s.logger.Error("ensure client", "ip", client.IP, "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no se pudo recuperar tu identificador"})
		return
	}

	s.writeSessionCookie(w, snapshot.ID)
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleChangeAnimal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}

	client := s.clientInfo(r)
	current, err := s.sessions.Ensure(client)
	if err != nil {
		s.logger.Error("ensure client before animal change", "ip", client.IP, "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no se pudo preparar tu identificador"})
		return
	}

	updated, err := s.sessions.RotateAnimal(client)
	if err != nil {
		s.logger.Error("change animal", "ip", client.IP, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "no se pudo cambiar el animal"})
		return
	}

	if err := s.uploads.MoveAnimal(current.Animal, updated.Animal); err != nil {
		s.logger.Error("move uploads after animal change", "ip", client.IP, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "el animal cambio, pero no se pudo mover la carpeta"})
		return
	}

	s.writeSessionCookie(w, updated.ID)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleAccess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}

	client := s.clientInfo(r)
	snapshot, err := s.sessions.Activate(r.Context(), client)
	if err != nil {
		s.logger.Error("activate access", "ip", client.IP, "error", err)
		fallback, ensureErr := s.sessions.Ensure(client)
		if ensureErr != nil {
			fallback = sessions.Snapshot{}
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error":        "no se pudo activar internet temporal",
			"details":      err.Error(),
			"animal":       fallback.Animal,
			"animal_name":  fallback.AnimalName,
			"identifier":   fallback.Identifier,
			"animal_emoji": fallback.AnimalEmoji,
		})
		return
	}

	s.writeSessionCookie(w, snapshot.ID)
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.uploads.MaxBytes())
	if err := r.ParseMultipartForm(s.uploads.MaxBytes()); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": fmt.Sprintf("el archivo excede el limite de %d MB", s.cfg.MaxUploadMB),
		})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "selecciona un archivo valido"})
		return
	}

	client := s.clientInfo(r)
	snapshot, err := s.sessions.Ensure(client)
	if err != nil {
		s.logger.Error("ensure client before upload", "ip", client.IP, "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no se pudo preparar tu carpeta"})
		return
	}
	s.writeSessionCookie(w, snapshot.ID)
	saved, err := s.uploads.Save(file, header, client.IP, snapshot.Animal)
	if err != nil {
		s.logger.Error("save upload", "ip", client.IP, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "no se pudo guardar el archivo"})
		return
	}

	s.snapshotUpload(client)
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleClientUploads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}

	client := s.clientInfo(r)
	snapshot, err := s.sessions.Ensure(client)
	if err != nil {
		s.logger.Error("ensure client before uploads list", "ip", client.IP, "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no se pudo recuperar tu carpeta"})
		return
	}
	s.writeSessionCookie(w, snapshot.ID)
	items, err := s.uploads.ListByAnimal(snapshot.Animal)
	if err != nil {
		s.logger.Error("list uploads", "ip", client.IP, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "no se pudo cargar la lista de archivos"})
		return
	}

	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleDeleteUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}

	var payload struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "solicitud invalida"})
		return
	}

	client := s.clientInfo(r)
	snapshot, err := s.sessions.Ensure(client)
	if err != nil {
		s.logger.Error("ensure client before delete upload", "ip", client.IP, "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no se pudo recuperar tu carpeta"})
		return
	}

	if err := s.uploads.DeleteByAnimal(snapshot.Animal, payload.Name); err != nil {
		if strings.Contains(err.Error(), "not exist") {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "el archivo ya no existe"})
			return
		}
		s.logger.Error("delete upload", "ip", client.IP, "file", payload.Name, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "no se pudo eliminar el archivo"})
		return
	}

	s.writeSessionCookie(w, snapshot.ID)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": payload.Name})
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	if !isLocalRequest(r) {
		http.NotFound(w, r)
		return
	}

	writeJSON(w, http.StatusOK, overviewResponse{
		Controller: s.controller.Name(),
		Mode:       s.cfg.Network.ActivationMode,
		Sessions:   s.sessions.List(),
		Uploads:    s.uploads.Recent(),
	})
}

func (s *Server) snapshotUpload(client sessions.ClientInfo) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if _, err := s.sessions.Touch(ctx, client); err != nil {
		s.logger.Warn("refresh session before upload", "ip", client.IP, "error", err)
	}
	if _, err := s.sessions.MarkUpload(client); err != nil {
		s.logger.Warn("mark upload", "ip", client.IP, "error", err)
	}
}

func (s *Server) clientInfo(r *http.Request) sessions.ClientInfo {
	clientIP := remoteIP(r)
	sessionID := ""
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		sessionID = cookie.Value
	}
	return sessions.ClientInfo{
		IP:        clientIP,
		MAC:       lookupMAC(r.Context(), clientIP),
		SessionID: sessionID,
	}
}

func (s *Server) writeSessionCookie(w http.ResponseWriter, sessionID string) {
	if strings.TrimSpace(sessionID) == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
	})
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("request", "method", r.Method, "path", r.URL.Path, "remote", remoteIP(r), "duration", time.Since(start))
	})
}

func (s *Server) methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "metodo no permitido"})
}

func remoteIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}

func lookupMAC(ctx context.Context, clientIP string) string {
	if clientIP == "" || runtime.GOOS != "windows" {
		return ""
	}

	cmd := exec.CommandContext(ctx, "arp", "-a", clientIP)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	match := macPattern.FindString(string(output))
	return strings.ToLower(match)
}

func isLocalRequest(r *http.Request) bool {
	ip := remoteIP(r)
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.IsLoopback()
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
