// Package toon encodes Go values into TOON (Token-Optimized Object Notation),
// a compact, LLM-friendly serialization format aligned with axi.md principle #1.
//
// TOON omits braces, quotes, and commas when unambiguous, typically saving
// ~40% tokens over equivalent JSON. Arrays of uniform objects are encoded
// in a tabular form that is especially compact:
//
//	issues[2]{number,title,state}:
//	  42,Fix login bug,open
//	  43,Add dark mode,open
//
// Supported inputs: nil, bool, integer and floating-point numbers, string,
// map[string]any, []any, and nested combinations thereof. Typed structs are
// not directly supported — marshal them through encoding/json to a
// map[string]any first, or pass values already shaped as map[string]any.
package toon

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Encode returns the TOON encoding of v.
//
// Top-level scalars produce a single line with no trailing newline. Top-level
// maps and slices produce a multi-line block; the returned string has no
// trailing newline.
func Encode(v any) (string, error) {
	var sb strings.Builder
	if err := encodeRoot(&sb, v); err != nil {
		return "", err
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}

func encodeRoot(sb *strings.Builder, v any) error {
	switch x := v.(type) {
	case nil:
		sb.WriteString("null")
		return nil
	case map[string]any:
		return encodeMap(sb, x, 0)
	case []any:
		return encodeSlice(sb, "", x, 0)
	default:
		if s, ok := scalar(v); ok {
			sb.WriteString(s)
			return nil
		}
		return fmt.Errorf("toon: unsupported top-level type %T (pass map[string]any, []any, or a scalar)", v)
	}
}

func encodeMap(sb *strings.Builder, m map[string]any, indent int) error {
	keys := sortedKeys(m)
	for _, k := range keys {
		if err := encodeEntry(sb, k, m[k], indent); err != nil {
			return err
		}
	}
	return nil
}

func encodeEntry(sb *strings.Builder, key string, v any, indent int) error {
	pad := pad(indent)
	safeKey := quoteIfNeeded(key)
	switch x := v.(type) {
	case map[string]any:
		if len(x) == 0 {
			fmt.Fprintf(sb, "%s%s:\n", pad, safeKey)
			return nil
		}
		fmt.Fprintf(sb, "%s%s:\n", pad, safeKey)
		return encodeMap(sb, x, indent+1)
	case []any:
		return encodeSlice(sb, safeKey, x, indent)
	default:
		s, ok := scalar(v)
		if !ok {
			return fmt.Errorf("toon: unsupported value type %T at key %q", v, key)
		}
		fmt.Fprintf(sb, "%s%s: %s\n", pad, safeKey, s)
		return nil
	}
}

func encodeSlice(sb *strings.Builder, key string, s []any, indent int) error {
	pad := pad(indent)
	if fields := uniformMapFields(s); fields != nil {
		safeFields := make([]string, len(fields))
		for i, f := range fields {
			safeFields[i] = quoteIfNeeded(f)
		}
		fmt.Fprintf(sb, "%s%s[%d]{%s}:\n", pad, key, len(s), strings.Join(safeFields, ","))
		for _, item := range s {
			m := item.(map[string]any)
			vals := make([]string, len(fields))
			for i, f := range fields {
				v, _ := scalar(m[f])
				vals[i] = v
			}
			fmt.Fprintf(sb, "%s  %s\n", pad, strings.Join(vals, ","))
		}
		return nil
	}
	if allScalar(s) {
		fmt.Fprintf(sb, "%s%s[%d]:\n", pad, key, len(s))
		for _, item := range s {
			v, _ := scalar(item)
			fmt.Fprintf(sb, "%s  %s\n", pad, v)
		}
		return nil
	}
	// Heterogeneous slice: emit as numbered entries.
	fmt.Fprintf(sb, "%s%s[%d]:\n", pad, key, len(s))
	for i, item := range s {
		if err := encodeEntry(sb, strconv.Itoa(i), item, indent+1); err != nil {
			return err
		}
	}
	return nil
}

// uniformMapFields returns the sorted field list if s is a non-empty slice of
// maps sharing identical key sets with scalar-only values; otherwise nil.
func uniformMapFields(s []any) []string {
	if len(s) == 0 {
		return nil
	}
	first, ok := s[0].(map[string]any)
	if !ok || len(first) == 0 {
		return nil
	}
	for _, v := range first {
		if !isScalar(v) {
			return nil
		}
	}
	fields := sortedKeys(first)
	for _, item := range s[1:] {
		m, ok := item.(map[string]any)
		if !ok || len(m) != len(fields) {
			return nil
		}
		for _, f := range fields {
			v, exists := m[f]
			if !exists || !isScalar(v) {
				return nil
			}
		}
	}
	return fields
}

func allScalar(s []any) bool {
	for _, item := range s {
		if !isScalar(item) {
			return false
		}
	}
	return true
}

func isScalar(v any) bool {
	if v == nil {
		return true
	}
	_, ok := scalar(v)
	return ok
}

// scalar formats a scalar value. Returns ok=false for non-scalar types.
func scalar(v any) (string, bool) {
	switch x := v.(type) {
	case nil:
		return "null", true
	case bool:
		if x {
			return "true", true
		}
		return "false", true
	case string:
		return quoteIfNeeded(x), true
	case int:
		return strconv.FormatInt(int64(x), 10), true
	case int8:
		return strconv.FormatInt(int64(x), 10), true
	case int16:
		return strconv.FormatInt(int64(x), 10), true
	case int32:
		return strconv.FormatInt(int64(x), 10), true
	case int64:
		return strconv.FormatInt(x, 10), true
	case uint:
		return strconv.FormatUint(uint64(x), 10), true
	case uint8:
		return strconv.FormatUint(uint64(x), 10), true
	case uint16:
		return strconv.FormatUint(uint64(x), 10), true
	case uint32:
		return strconv.FormatUint(uint64(x), 10), true
	case uint64:
		return strconv.FormatUint(x, 10), true
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32), true
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	default:
		return "", false
	}
}

// quoteIfNeeded wraps s in double quotes when raw emission would be ambiguous.
// A bare string is safe when it has no structural characters (`:` `,` newline
// `"` `\`), no leading/trailing whitespace, and is not a reserved token or a
// number-like literal.
func quoteIfNeeded(s string) string {
	if s == "" {
		return `""`
	}
	if needsQuote(s) {
		return strconv.Quote(s)
	}
	return s
}

func needsQuote(s string) bool {
	// Invalid UTF-8 must be escaped via strconv.Quote, otherwise the encoder
	// emits byte sequences that agents cannot parse as text.
	if !utf8.ValidString(s) {
		return true
	}
	switch s {
	case "null", "true", "false":
		return true
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return true
	}
	if s[0] == ' ' || s[0] == '\t' || s[len(s)-1] == ' ' || s[len(s)-1] == '\t' {
		return true
	}
	for _, r := range s {
		switch r {
		case ':', ',', '\n', '\r', '"', '\\':
			return true
		}
		// Any control character (including NUL, tab, DEL) forces quoting so
		// it can be safely escaped by strconv.Quote.
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func pad(n int) string {
	return strings.Repeat("  ", n)
}
