package contracts

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHandleA2ATask_Happy(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/a2a/tasks", strings.NewReader(`{"taskId":"t1","agent":"developer","payload":"fix vuln"}`))
	res := httptest.NewRecorder()

	HandleA2ATask(res, req)

	require.Equal(t, http.StatusAccepted, res.Code)
	require.Contains(t, res.Body.String(), "queued")
}

func TestHandleA2ATask_UnhappyValidation(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/a2a/tasks", strings.NewReader(`{"taskId":"","agent":"developer","payload":""}`))
	res := httptest.NewRecorder()

	HandleA2ATask(res, req)

	require.Equal(t, http.StatusBadRequest, res.Code)
}

func TestHandleMCPTool_Happy(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools", strings.NewReader(`{"toolName":"run_gosec_scan","arguments":{"path":"."}}`))
	res := httptest.NewRecorder()

	HandleMCPTool(res, req)

	require.Equal(t, http.StatusOK, res.Code)
	require.Contains(t, res.Body.String(), "ok")
}

func TestHandleMCPTool_UnhappyValidation(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools", strings.NewReader(`{"toolName":"","arguments":{}}`))
	res := httptest.NewRecorder()

	HandleMCPTool(res, req)

	require.Equal(t, http.StatusBadRequest, res.Code)
}
