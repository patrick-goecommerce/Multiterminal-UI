package mcpsrv

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// The handlers are glue, so what is worth testing about them is that a bad
// call comes back as a tool error a model can read, and not as a transport
// error that reaches it as nothing at all.

func toolReq(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

func TestHandleOpenSessionRequiresToolAndDir(t *testing.T) {
	s, _ := testServer(t)
	res, err := s.handleOpenSession(context.Background(), toolReq(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result when tool/dir are missing")
	}
}

func TestHandleOpenSessionRejectsUnsupportedTool(t *testing.T) {
	s, _ := testServer(t)
	res, err := s.handleOpenSession(context.Background(), toolReq(map[string]any{
		"tool": "notreal",
		"dir":  t.TempDir(),
	}))
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result for an unsupported tool")
	}
}

func TestHandleSendInputRequiresSessionID(t *testing.T) {
	s, _ := testServer(t)
	res, err := s.handleSendInput(context.Background(), toolReq(map[string]any{"text": "hi"}))
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result when session_id is missing")
	}
}

func TestHandleCloseSessionUnknownSession(t *testing.T) {
	s, _ := testServer(t)
	res, err := s.handleCloseSession(context.Background(), toolReq(map[string]any{
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
	s, _ := testServer(t)
	res, err := s.handleListSessions(context.Background(), toolReq(nil))
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %+v", res)
	}
}
