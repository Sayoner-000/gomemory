package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"

	"mem/store"
)

type Server struct {
	db       *sql.DB
	project  string
	port     int
	listener net.Listener
	mux      *http.ServeMux
}

type SessionStartResponse struct {
	SessionID string `json:"session_id"`
	CreatedAt string `json:"created_at"`
}

type SessionEndResponse struct {
	SessionID string `json:"session_id"`
	EndedAt   string `json:"ended_at"`
}

type ContextResponse struct {
	ActiveSession  bool            `json:"active_session"`
	RecentSessions []SessionBrief  `json:"recent_sessions"`
	RecentMemories []MemoryBrief   `json:"recent_memories"`
}

type SessionBrief struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Created string `json:"created_at"`
	Ended   string `json:"ended_at,omitempty"`
}

type MemoryBrief struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
	Created string `json:"created_at"`
}

func New(db *sql.DB, project string, port int) *Server {
	mux := http.NewServeMux()
	s := &Server{db: db, project: project, port: port, mux: mux}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("POST /session/start", s.handleSessionStart)
	s.mux.HandleFunc("POST /session/end", s.handleSessionEnd)
	s.mux.HandleFunc("GET /context", s.handleContext)
	s.mux.HandleFunc("GET /health", s.handleHealth)
}

func (s *Server) Start() error {
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(s.port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("server listen on %s: %w", addr, err)
	}
	s.listener = listener
	log.Printf("gomemory server listening on %s", addr)
	return http.Serve(listener, s.mux)
}

func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *Server) handleSessionStart(w http.ResponseWriter, r *http.Request) {
	active, err := store.ActiveSession(s.db, s.project)
	if err != nil {
		jsonError(w, "error checking active session: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if active != nil {
		jsonResp(w, SessionStartResponse{
			SessionID: active.ID,
			CreatedAt: active.CreatedAt,
		})
		return
	}

	session, err := store.StartSession(s.db, s.project)
	if err != nil {
		jsonError(w, "error starting session: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, SessionStartResponse{
		SessionID: session.ID,
		CreatedAt: session.CreatedAt,
	})
}

func (s *Server) handleSessionEnd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"session_id"`
		Summary   string `json:"summary"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		active, err := store.ActiveSession(s.db, s.project)
		if err != nil {
			jsonError(w, "error finding active session: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if active == nil {
			jsonError(w, "no active session found", http.StatusNotFound)
			return
		}
		req.SessionID = active.ID
	}

	if err := store.EndSession(s.db, req.SessionID, req.Summary); err != nil {
		jsonError(w, "error ending session: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResp(w, SessionEndResponse{
		SessionID: req.SessionID,
		EndedAt:   store.Now,
	})
}

func (s *Server) handleContext(w http.ResponseWriter, r *http.Request) {
	active, _ := store.ActiveSession(s.db, s.project)
	recentSessions, _ := store.RecentSessions(s.db, s.project, 5)
	recentMemories, _ := store.ListMemories(s.db, s.project, 10)

	sessions := make([]SessionBrief, 0, len(recentSessions))
	for _, sess := range recentSessions {
		brief := SessionBrief{
			ID:      sess.ID,
			Summary: sess.Summary,
			Created: sess.CreatedAt,
		}
		if sess.EndedAt != nil {
			brief.Ended = *sess.EndedAt
		}
		sessions = append(sessions, brief)
	}

	memories := make([]MemoryBrief, 0, len(recentMemories))
	for _, m := range recentMemories {
		memories = append(memories, MemoryBrief{
			ID:      m.ID,
			Title:   m.Title,
			Type:    string(m.Type),
			Created: m.CreatedAt,
		})
	}

	jsonResp(w, ContextResponse{
		ActiveSession:  active != nil,
		RecentSessions: sessions,
		RecentMemories: memories,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonResp(w, map[string]string{"status": "ok"})
}

func jsonResp(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
