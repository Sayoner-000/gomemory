package main

import (
	"bytes"
	"testing"
)

func TestHCD014ControlledRouteCapture(t *testing.T) {
	for _, body := range []string{
		`{ "model":"fixture", "input":[{"role":"system","content":"KEEP"},{"type":"function_call","call_id":"c1","name":"exec_command","arguments":"{\"cmd\":\"go test ./...\"}"},{"type":"function_call_output","call_id":"c1","output":"ok"}], "stream":true }`,
		`{ "model":"fixture", "messages":[{"role":"system","content":"KEEP"},{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"shell","arguments":"{\"command\":\"go test ./...\"}"}}]},{"role":"tool","tool_call_id":"c1","content":"ok"}], "stream":true }`,
		`{ "model":"fixture", "system":"KEEP", "messages":[{"role":"assistant","content":[{"type":"tool_use","id":"c1","name":"Bash","input":{"command":"go test ./..."}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"c1","content":"ok"}]}], "stream":true }`,
	} {
		input := []byte(body)
		got := hcd014Capture(t, input, nil)
		if !bytes.Equal(got, input) {
			t.Fatalf("passthrough changed the literal request: %s", got)
		}
	}
}
