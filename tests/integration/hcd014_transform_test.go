package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"mem/adapters/secondary/clock"
	"mem/adapters/secondary/compression/native"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/requestcompression"
	"mem/application/ports"
)

func TestHCD014ControlledRouteTransform(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	engine := native.NewEngine(persistence.NewOriginalStoreRepository(db, clock.SystemClock{}, 0, 0), nil, nil, "hcd014")
	var logs strings.Builder
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&logs, "2026-11-02 10:00:%02d INFO worker_%03d heartbeat completed successfully job=%d\n", i%60, i, i)
	}
	log := logs.String()
	control, controlErr := engine.Compress(log, ports.CompressionOptions{Level: ports.CompressionMax, ToolOutput: true})
	if controlErr != nil || !control.Compressed {
		t.Fatalf("fixture not reducible: err=%v fallback=%s refs=%v", controlErr, control.FallbackReason, control.Refs)
	}
	q := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	for _, tc := range []struct{ format, body string }{
		{"responses", `{ "instructions" : "KEEP EXACT", "input" : [{"role":"user","content":"KEEP USER"}, {"type":"function_call","call_id":"c1","name":"exec_command","arguments":"{\"cmd\":\"go test ./...\"}"}, {"type":"function_call_output","call_id":"c1","output":` + q(log) + `,"unknown":{"number":9007199254740993}}, {"role":"user","content":"NEXT"}], "unknown":true }`},
		{"chat", `{ "messages" : [{"role":"system","content":"KEEP EXACT"},{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"shell","arguments":"{\"command\":\"go test ./...\"}"}}]},{"role":"tool","tool_call_id":"c1","content":` + q(log) + `},{"role":"user","content":"NEXT"}], "unknown":9007199254740993 }`},
		{"messages", `{ "system" : "KEEP EXACT", "messages" : [{"role":"assistant","content":[{"type":"tool_use","id":"c1","name":"Bash","input":{"command":"go test ./..."}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"c1","content":` + q(log) + `},{"type":"text","text":"KEEP USER"}]},{"role":"user","content":"NEXT"}], "unknown":9007199254740993 }`},
	} {
		t.Run(tc.format, func(t *testing.T) {
			body := []byte(tc.body)
			before := bytes.Clone(body)
			out := hcd014Capture(t, body, func(b []byte) []byte { return requestcompression.Transform(b, tc.format, engine) })
			if !bytes.Equal(body, before) {
				t.Fatal("host original mutated")
			}
			if !json.Valid(out) || len(out) >= len(body) || !bytes.Contains(out, []byte("mem pack retrieve")) {
				t.Fatalf("no recoverable reduction: input=%d output=%d valid=%v suffix=%.200s", len(body), len(out), json.Valid(out), out[max(0, len(out)-200):])
			}
			// Only the result string may change: the entire surrounding wire prefix
			// and suffix (including unknown fields, numeric literals and whitespace)
			// must be byte-identical.
			parts := bytes.Split(body, []byte(q(log)))
			if len(parts) != 2 || !bytes.HasPrefix(out, parts[0]) || !bytes.HasSuffix(out, parts[1]) {
				t.Fatal("protocol/unknown fields or stable prefix changed")
			}
			for name, b := range map[string][]byte{
				"exact_read":     bytes.ReplaceAll(body, []byte("go test ./..."), []byte("cat source.go")),
				"unknown_args":   bytes.ReplaceAll(body, []byte("go test ./..."), []byte("")),
				"small":          bytes.ReplaceAll(body, []byte(q(log)), []byte(`"ok"`)),
				"unmatched":      bytes.Replace(body, []byte(`"c1"`), []byte(`"different"`), 1),
				"compound_read":  bytes.ReplaceAll(body, []byte("go test ./..."), []byte("go test ./...; cat source.go")),
				"nontext_result": bytes.ReplaceAll(body, []byte(q(log)), []byte(`[{"type":"image","source":{"data":"KEEP"}}]`)),
				"duplicate_key":  append([]byte(`{"unknown":false,`), body[1:]...),
			} {
				if got := requestcompression.Transform(b, tc.format, engine); !bytes.Equal(got, b) {
					t.Errorf("%s must pass through", name)
				}
			}
			if got := requestcompression.Transform(body, "unknown", engine); !bytes.Equal(got, body) {
				t.Fatal("unknown protocol changed")
			}
		})
	}
}
