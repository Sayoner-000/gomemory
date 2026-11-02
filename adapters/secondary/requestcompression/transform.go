// Package requestcompression evaluates request-body reduction. It has no
// listener, forwarding destination, credentials or activation side effects.
package requestcompression

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strings"

	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

type call struct {
	eligible  bool
	position  int
	duplicate bool
}
type result struct {
	id       string
	text     *node
	position int
}
type edit struct {
	start, end  int
	replacement []byte
}

// Transform replaces only large textual results of recognized shell calls.
// Unknown protocols, shapes, ambiguous IDs and failed/nonrecoverable reductions
// pass through. The caller retains the host's original request unchanged.
// This adapter deliberately does not compress ordinary conversation prose.
func Transform(body []byte, format string, compressor ports.Compressor) []byte {
	return transform(body, format, compressor, nil)
}

// SessionAccess identifies the host session; it must survive compaction/resume.
// The adapter never substitutes a process-local identity for a missing host ID.
type SessionAccess struct {
	Originals ports.OriginalStoreRepository
	Grants    ports.AutomaticOriginalGrantsRepository
	Scope     ports.AutomaticOriginalScope
}

// TransformForSession publishes references only after granting the owning
// session access to persisted originals. Missing identity/storage passes through.
func TransformForSession(ctx context.Context, body []byte, format string, compressor ports.Compressor, access SessionAccess) []byte {
	return transform(body, format, compressor, func(refs []string) (string, error) {
		if err := usecases.GrantAutomaticOriginals(ctx, access.Originals, access.Grants, access.Scope, refs); err != nil {
			return "", err
		}
		// Single-quote the opaque host ID: double quotes would still permit shell
		// expansion of dollars and backticks in a recovery command.
		session := "'" + strings.ReplaceAll(access.Scope.SessionID, "'", "'\"'\"'") + "'"
		return "mem pack retrieve --automatic --agent " + access.Scope.Agent + " --session-id " + session + " <ref>", nil
	})
}

func transform(body []byte, format string, compressor ports.Compressor, grant func([]string) (string, error)) []byte {
	if compressor == nil || len(body) > 8<<20 {
		return body
	}
	if format != "responses" && format != "chat" && format != "messages" {
		return body
	}
	root, err := parse(body)
	if err != nil || root.object == nil {
		return body
	}
	calls := map[string]call{}
	var results []result
	position := 0
	addCall := func(id, name string, args *node) {
		position++
		if id == "" {
			return
		}
		if c, exists := calls[id]; exists {
			c.duplicate = true
			calls[id] = c
			return
		}
		calls[id] = call{eligible: eligible(name, args), position: position}
	}
	addResult := func(id string, n *node) {
		position++
		if n != nil {
			if _, ok := n.value.(string); ok {
				results = append(results, result{id, n, position})
			}
		}
	}
	switch format {
	case "responses":
		for _, item := range root.field("input").items() {
			switch item.field("type").text() {
			case "function_call":
				addCall(item.field("call_id").text(), item.field("name").text(), arguments(item.field("arguments")))
			case "function_call_output":
				addResult(item.field("call_id").text(), item.field("output"))
			}
		}
	case "chat":
		for _, message := range root.field("messages").items() {
			switch message.field("role").text() {
			case "assistant":
				for _, tc := range message.field("tool_calls").items() {
					if tc.field("type").text() != "function" {
						continue
					}
					f := tc.field("function")
					addCall(tc.field("id").text(), f.field("name").text(), arguments(f.field("arguments")))
				}
			case "tool":
				addResult(message.field("tool_call_id").text(), message.field("content"))
			}
		}
	case "messages":
		for _, message := range root.field("messages").items() {
			for _, block := range message.field("content").items() {
				if message.field("role").text() == "assistant" && block.field("type").text() == "tool_use" {
					addCall(block.field("id").text(), block.field("name").text(), block.field("input"))
				}
				if message.field("role").text() == "user" && block.field("type").text() == "tool_result" {
					addResult(block.field("tool_use_id").text(), block.field("content"))
				}
			}
		}
	}
	counts := map[string]int{}
	for _, r := range results {
		counts[r.id]++
	}
	var edits []edit
	for _, r := range results {
		c, ok := calls[r.id]
		if !ok || !c.eligible || c.duplicate || counts[r.id] != 1 || c.position >= r.position {
			continue
		}
		original := r.text.text()
		if len(original) < 8000 {
			continue
		}
		compressed, e := compressor.Compress(original, ports.CompressionOptions{Level: ports.CompressionMax, ToolOutput: true, PreserveCode: true, PreserveURLs: true, PreservePaths: true, PreserveErrors: true})
		if e != nil || !compressed.Compressed || len(compressed.Refs) == 0 || compressed.Content == "" {
			continue
		}
		// Recovery instructions and references are part of the delivered cost.
		recovery := "mem pack retrieve <ref>"
		if grant != nil {
			var err error
			recovery, err = grant(compressed.Refs)
			if err != nil {
				continue
			}
		}
		delivered := compressed.Content + "\n[gomemory: refs=" + strings.Join(compressed.Refs, ",") + "; recuperar cada original con " + recovery + "]"
		if len(delivered) >= len(original) {
			continue
		}
		replacement, e := json.Marshal(delivered)
		if e != nil {
			continue
		}
		if len(replacement) >= r.text.end-r.text.start {
			continue
		}
		edits = append(edits, edit{r.text.start, r.text.end, replacement})
	}
	if len(edits) == 0 {
		return body
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var out bytes.Buffer
	previous := 0
	for _, e := range edits {
		out.Write(body[previous:e.start])
		out.Write(e.replacement)
		previous = e.end
	}
	out.Write(body[previous:])
	return out.Bytes()
}

func arguments(n *node) *node {
	if n == nil {
		return nil
	}
	root, e := parse([]byte(n.text()))
	if e != nil {
		return nil
	}
	return root
}

func eligible(name string, args *node) bool {
	var command string
	switch name {
	case "exec_command":
		command = args.field("cmd").text()
	case "Bash", "bash", "shell":
		command = args.field("command").text()
	default:
		return false
	}
	if strings.TrimSpace(command) == "" || domain.IsExactReadCommand(command) {
		return false
	}
	// Compound commands can hide exact reads after a pipe, substitution or
	// separator. Until parsed by the host they are outside this evaluated scope.
	return !strings.ContainsAny(command, ";|&\n`$<>")
}
