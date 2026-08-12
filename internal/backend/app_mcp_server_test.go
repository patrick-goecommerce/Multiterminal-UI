package backend

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func toolReq(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

func TestHandleOpenSessionRequiresToolAndDir(t *testing.T) {
	a := newTestAgentControlService()

	res, err := a.handleOpenSession(context.Background(), toolReq(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result when tool/dir are missing")
	}
}

func TestHandleOpenSessionRejectsUnsupportedTool(t *testing.T) {
	a := newTestAgentControlService()

	res, err := a.handleOpenSession(context.Background(), toolReq(map[string]any{
		"tool": "notreal",
		"dir":  "C:\\tmp",
	}))
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result for an unsupported tool")
	}
}

func TestHandleSendInputRequiresSessionID(t *testing.T) {
	a := newTestAgentControlService()

	res, err := a.handleSendInput(context.Background(), toolReq(map[string]any{"text": "hi"}))
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result when session_id is missing")
	}
}

func TestHandleCloseSessionUnknownSession(t *testing.T) {
	a := newTestAgentControlService()

	res, err := a.handleCloseSession(context.Background(), toolReq(map[string]any{
		"session_id": float64(999),
	}))
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result for an unknown session")
	}
}

func TestHandleListSessionsEmpty(t *testing.T) {
	a := newTestAgentControlService()

	res, err := a.handleListSessions(context.Background(), toolReq(nil))
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %+v", res)
	}
}

func TestStartMCPServerBindsToLoopback(t *testing.T) {
	// startMCPServer publishes its port; without the redirect the test would
	// overwrite the record of the developer's own running instance.
	useTempDiscoveryDir(t)
	a := newTestAgentControlService()

	port, err := a.startMCPServer(0)
	if err != nil {
		t.Fatalf("startMCPServer: %v", err)
	}
	if port <= 0 {
		t.Fatalf("expected a bound port, got %d", port)
	}
}
