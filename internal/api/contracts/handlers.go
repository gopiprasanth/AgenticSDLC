package contracts

import (
	"encoding/json"
	"errors"
	"net/http"
)

type A2ATaskRequest struct {
	TaskID   string `json:"taskId"`
	Agent    string `json:"agent"`
	Payload  string `json:"payload"`
	Priority string `json:"priority"`
}

type MCPToolRequest struct {
	ToolName  string            `json:"toolName"`
	Arguments map[string]string `json:"arguments"`
}

func (r A2ATaskRequest) Validate() error {
	if r.TaskID == "" || r.Agent == "" || r.Payload == "" {
		return errors.New("taskId, agent and payload are required")
	}
	return nil
}

func (r MCPToolRequest) Validate() error {
	if r.ToolName == "" {
		return errors.New("toolName is required")
	}
	return nil
}

func HandleA2ATask(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	var payload A2ATaskRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := payload.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"state":"queued"}`))
}

func HandleMCPTool(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	var payload MCPToolRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := payload.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
