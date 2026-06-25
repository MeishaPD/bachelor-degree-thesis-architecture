package httpadapter

import "net/http"

type PingHandler struct{}

func NewPingHandler() *PingHandler { return &PingHandler{} }

func (h *PingHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method tidak diizinkan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "200 OK (Healthy)", "service": "hexagonal-backend"})
}
