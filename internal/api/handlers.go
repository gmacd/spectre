package api

import (
	"encoding/json"
	"net/http"
)

type sendRequest struct {
	ConversationID string `json:"conversation_id"`
	Message        string `json:"message"`
}

type sendResponse struct {
	ConversationID string `json:"conversation_id"`
	Reply          string `json:"reply"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type healthResponse struct {
	Status        string `json:"status"`
	DB            string `json:"db"`
	LLMConfigured bool   `json:"llm_configured"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

// handleHealth is intentionally cheap: it pings the DB but does not
// round-trip to the LLM backend, so daemon liveness isn't coupled to LLM
// availability.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	dbStatus := "ok"
	if err := s.db.Ping(r.Context()); err != nil {
		dbStatus = "error: " + err.Error()
	}
	writeJSON(w, http.StatusOK, healthResponse{
		Status:        "ok",
		DB:            dbStatus,
		LLMConfigured: s.llmConfigured,
	})
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	var req sendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.ConversationID == "" || req.Message == "" {
		writeError(w, http.StatusBadRequest, "conversation_id and message are required")
		return
	}

	reply, err := s.agent.HandleMessage(r.Context(), req.ConversationID, req.Message)
	if err != nil {
		s.logger.Error("handle message failed", "conversation_id", req.ConversationID, "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, sendResponse{ConversationID: req.ConversationID, Reply: reply})
}
