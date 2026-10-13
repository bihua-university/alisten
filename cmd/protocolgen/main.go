// protocolgen generates client/server protocol bindings from an IDL written
// in JSON Schema form (protocol/ws.idl.json). Generators render the Go DTOs
// and message registries, and (optionally) TypeScript types.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Schema is the subset of JSON Schema (plus x-go-name metadata) the generators
// understand. Structural keywords only: value constraints such as enum,
// minLength or minimum are documentation for handlers and clients, which own
// their own input validation.
type schema struct {
	Ref        string            `json:"$ref"`
	Type       string            `json:"type"`
	Enum       []string          `json:"enum"`
	Properties map[string]schema `json:"properties"`
	Required   []string          `json:"required"`
	Items      *schema           `json:"items"`
	AnyOf      []schema          `json:"anyOf"`
	GoName     string            `json:"x-go-name"`
}

type request struct{ Type, GoName, Request, Response string }
type event struct{ Type, GoName, Data string }

// IDL is the parsed protocol definition shared by all generators.
type IDL struct {
	Defs     map[string]schema
	Requests []request
	Events   []event
}

// loadIDL reads and parses an IDL file (JSON Schema form with x-requests,
// x-events and $defs).
func loadIDL(path string) (*IDL, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var root struct {
		schema   `json:",inline"`
		Defs     map[string]schema `json:"$defs"`
		Requests []request         `json:"x-requests"`
		Events   []event           `json:"x-events"`
	}
	if err := json.Unmarshal(b, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &IDL{Defs: root.Defs, Requests: root.Requests, Events: root.Events}, nil
}

// defNames returns the sorted $defs keys so output is deterministic.
func (i *IDL) defNames() []string { return keys(i.Defs) }

func exported(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '.' || r == '/' })
	for i, p := range parts {
		if p == strings.ToUpper(p) && len(p) > 1 {
			p = strings.ToLower(p)
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}

func keys(m map[string]schema) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func contains(values []string, v string) bool {
	for _, item := range values {
		if item == v {
			return true
		}
	}
	return false
}

// unsupported builds the standard error for IDL constructs no generator handles.
func unsupported(where, what string) error {
	return fmt.Errorf("%s: unsupported %s", where, what)
}

// Generator renders language-specific bindings from the parsed IDL.
// Implementations must be side effect free and produce deterministic output
// for a given IDL, so that -check can diff against committed files.
type Generator interface {
	// Name identifies the generator for diagnostics and flags.
	Name() string
	// Generate produces the output payloads for the given IDL.
	Generate(idl *IDL) ([]Output, error)
}

// Output is one artifact a generator produced; the CLI assigns the path.
type Output struct {
	Name string // descriptive file name for diagnostics
	Data []byte
}

// target pairs a generator with its output path; an empty path disables it.
type target struct {
	gen  Generator
	path string
}

func main() {
	idlPath := flag.String("idl", "protocol/ws.idl.json", "IDL source (JSON Schema form)")
	goPath := flag.String("go", "protocol/types.gen.go", "Go output path (empty disables the Go generator)")
	tsPath := flag.String("ts", "", "TypeScript output path (optional, disabled by default)")
	check := flag.Bool("check", false, "fail if a generated file differs from the target")
	flag.Parse()

	idl, err := loadIDL(*idlPath)
	must(err)

	targets := []target{
		{gen: goGenerator{}, path: *goPath},
		{gen: tsGenerator{}, path: *tsPath},
	}
	for _, t := range targets {
		if t.path == "" {
			continue
		}
		outputs, err := t.gen.Generate(idl)
		must(err)
		for _, out := range outputs {
			if len(outputs) == 1 {
				out.Name = t.path // single-output generators take the flag path verbatim
			}
			must(writeFile(out.Name, out.Data, *check))
		}
	}
}

func writeFile(path string, data []byte, check bool) error {
	if check {
		old, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(old, data) {
			return fmt.Errorf("generated file is stale: %s", path)
		}
		return nil
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0644)
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "protocolgen:", err)
		os.Exit(1)
	}
}
