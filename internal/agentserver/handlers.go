package agentserver

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type Handlers struct {
	pm       *ProcessManager
	topoResp *TopologyResponse
}

func NewHandlers(pm *ProcessManager, topoResp *TopologyResponse) *Handlers {
	return &Handlers{pm: pm, topoResp: topoResp}
}

func (h *Handlers) GetTopology(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.topoResp)
}

func (h *Handlers) PostJob(w http.ResponseWriter, r *http.Request) {
	var spec JobSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid json: " + err.Error()})
		return
	}

	if spec.ID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "id is required"})
		return
	}
	if spec.Command == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "command is required"})
		return
	}

	if err := h.pm.Start(spec); err != nil {
		writeJSON(w, http.StatusConflict, ErrorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": spec.ID})
}

func (h *Handlers) GetStatus(w http.ResponseWriter, r *http.Request) {
	jobs := h.pm.Status()
	writeJSON(w, http.StatusOK, StatusResponse{Jobs: jobs})
}

func (h *Handlers) GetMonitor(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, CollectMonitor())
}

func (h *Handlers) GetLogs(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "job id required"})
		return
	}

	var offset int64
	if s := r.URL.Query().Get("offset"); s != "" {
		var err error
		offset, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid offset"})
			return
		}
	}

	chunk, err := h.pm.ReadLog(jobID, offset)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, chunk)
}

func (h *Handlers) DeleteJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "job id required"})
		return
	}

	if err := h.pm.Kill(jobID); err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "killed"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
