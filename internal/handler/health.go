package handler

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"status":   "ok",
		"sessions": h.manager.Count(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
