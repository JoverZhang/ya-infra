package main

import (
	"bytes"
	"encoding/json"
	"errors"
)

type tunnelConfiguration struct {
	Source  string        `json:"source"`
	Version int           `json:"version"`
	Config  *tunnelConfig `json:"config"`
}

type tunnelConfig struct {
	Ingress []ingressRule `json:"ingress"`
	options map[string]any
}

type ingressRule struct {
	Hostname *string `json:"hostname,omitempty"`
	Path     *string `json:"path,omitempty"`
	Service  string  `json:"service"`
	options  map[string]any
}

type dnsRecord struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	TTL     int    `json:"ttl,omitempty"`
	Comment string `json:"comment,omitempty"`
	options map[string]any
}

// Keep provider options at the JSON boundary. Numbers must retain their precision.
func decodeProviderObject(data []byte, known any) (map[string]any, error) {
	if err := json.Unmarshal(data, known); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var fields map[string]any
	if err := decoder.Decode(&fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, errors.New("provider object must not be null")
	}
	return fields, nil
}

func encodeProviderObject(known any, options map[string]any) ([]byte, error) {
	data, err := json.Marshal(known)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	for name, value := range options {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		fields[name] = encoded
	}
	return json.Marshal(fields)
}

func (config *tunnelConfig) UnmarshalJSON(data []byte) error {
	type fields tunnelConfig
	var known fields
	options, err := decodeProviderObject(data, &known)
	if err != nil {
		return err
	}
	delete(options, "ingress")
	if len(options) == 0 {
		options = nil
	}
	*config = tunnelConfig(known)
	config.options = options
	return nil
}

func (config tunnelConfig) MarshalJSON() ([]byte, error) {
	type fields tunnelConfig
	return encodeProviderObject(fields(config), config.options)
}

func (rule *ingressRule) UnmarshalJSON(data []byte) error {
	type fields ingressRule
	var known fields
	options, err := decodeProviderObject(data, &known)
	if err != nil {
		return err
	}
	for _, name := range []string{"hostname", "path", "service"} {
		// Preserve explicit null on optional fields of unrelated rules.
		if options[name] != nil {
			delete(options, name)
		}
	}
	if len(options) == 0 {
		options = nil
	}
	*rule = ingressRule(known)
	rule.options = options
	return nil
}

func (rule ingressRule) MarshalJSON() ([]byte, error) {
	type fields ingressRule
	return encodeProviderObject(fields(rule), rule.options)
}

func (record *dnsRecord) UnmarshalJSON(data []byte) error {
	type fields dnsRecord
	var known fields
	options, err := decodeProviderObject(data, &known)
	if err != nil {
		return err
	}
	for _, name := range []string{"id", "name", "type", "content", "proxied", "ttl", "comment"} {
		delete(options, name)
	}
	if len(options) == 0 {
		options = nil
	}
	*record = dnsRecord(known)
	record.options = options
	return nil
}

func (record dnsRecord) MarshalJSON() ([]byte, error) {
	type fields dnsRecord
	return encodeProviderObject(fields(record), record.options)
}
