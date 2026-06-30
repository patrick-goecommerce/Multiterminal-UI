package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// postCapture POSTs the raw statusline JSON to MTUI. Silent on any failure.
func postCapture(payload []byte, port, sid string) {
	if port == "" || sid == "" {
		return
	}
	id, err := strconv.Atoi(sid)
	if err != nil {
		return
	}
	body, err := json.Marshal(struct {
		SessionID int             `json:"sessionId"`
		Payload   json.RawMessage `json:"payload"`
	}{SessionID: id, Payload: json.RawMessage(payload)})
	if err != nil {
		return
	}
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Post("http://127.0.0.1:"+port+"/api/statusline",
		"application/json", bytes.NewReader(body))
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}
