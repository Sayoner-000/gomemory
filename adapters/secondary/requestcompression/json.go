package requestcompression

import (
	"bytes"
	"encoding/json"
	"errors"
)

// node holds wire offsets so changing a tool result never reserializes the
// surrounding conversation, numeric literals, unknown fields or cache prefix.
type node struct {
	start, end int
	value      any
	object     map[string]*node
	array      []*node
}

func (n *node) field(key string) *node {
	if n == nil {
		return nil
	}
	return n.object[key]
}
func (n *node) text() string {
	if n == nil {
		return ""
	}
	s, _ := n.value.(string)
	return s
}
func (n *node) items() []*node {
	if n == nil {
		return nil
	}
	return n.array
}

func parse(body []byte) (*node, error) {
	if !json.Valid(body) {
		return nil, errors.New("invalid JSON")
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	return readNode(d, body)
}

func readNode(d *json.Decoder, body []byte) (*node, error) {
	start := int(d.InputOffset())
	for start < len(body) && bytes.ContainsRune([]byte(" \r\n\t:,"), rune(body[start])) {
		start++
	}
	token, err := d.Token()
	if err != nil {
		return nil, err
	}
	n := &node{start: start, value: token}
	switch token {
	case json.Delim('{'):
		n.object = map[string]*node{}
		for d.More() {
			key, e := d.Token()
			if e != nil {
				return nil, e
			}
			k, ok := key.(string)
			if !ok {
				return nil, errors.New("invalid key")
			}
			if _, exists := n.object[k]; exists {
				return nil, errors.New("duplicate key")
			}
			child, e := readNode(d, body)
			if e != nil {
				return nil, e
			}
			n.object[k] = child
		}
		if _, err = d.Token(); err != nil {
			return nil, err
		}
	case json.Delim('['):
		for d.More() {
			child, e := readNode(d, body)
			if e != nil {
				return nil, e
			}
			n.array = append(n.array, child)
		}
		if _, err = d.Token(); err != nil {
			return nil, err
		}
	}
	n.end = int(d.InputOffset())
	return n, nil
}
