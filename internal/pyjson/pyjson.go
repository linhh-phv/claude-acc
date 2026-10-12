// Package pyjson reads JSON keeping key order, and writes it back exactly as
// Python's json.dump(d, indent=2, ensure_ascii=False) did in the bash version
// of claude-acc — so switching implementations does not reformat a user's
// settings.json.
package pyjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// Number keeps a number as written, so integers of any size round-trip.
type Number string

// Object is a JSON object that remembers the order of its keys.
type Object struct {
	keys []string
	vals map[string]any
}

// NewObject returns an empty object.
func NewObject() *Object { return &Object{vals: map[string]any{}} }

// Get returns the value for k.
func (o *Object) Get(k string) (any, bool) {
	v, ok := o.vals[k]
	return v, ok
}

// Has reports whether k is present.
func (o *Object) Has(k string) bool { _, ok := o.vals[k]; return ok }

// Set stores v under k. An existing key keeps its position, like a Python dict.
func (o *Object) Set(k string, v any) {
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

// Delete removes k.
func (o *Object) Delete(k string) {
	if _, ok := o.vals[k]; !ok {
		return
	}
	delete(o.vals, k)
	for i, key := range o.keys {
		if key == k {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			break
		}
	}
}

// Keys returns the keys in order.
func (o *Object) Keys() []string { return append([]string(nil), o.keys...) }

// Len is the number of keys.
func (o *Object) Len() int { return len(o.keys) }

// String returns the string stored under k, or "".
func (o *Object) String(k string) string {
	s, _ := o.vals[k].(string)
	return s
}

// Parse decodes one JSON value. Objects come back as *Object, arrays as []any,
// numbers as Number. A leading UTF-8 BOM is accepted (Windows editors add one).
func Parse(data []byte) (any, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := parseValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("extra data after JSON value")
	}
	return v, nil
}

// ParseObject decodes data that must be a JSON object.
func ParseObject(data []byte) (*Object, error) {
	v, err := Parse(data)
	if err != nil {
		return nil, err
	}
	o, ok := v.(*Object)
	if !ok {
		return nil, errors.New("JSON value is not an object")
	}
	return o, nil
}

func parseValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			o := NewObject()
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				k, _ := kt.(string)
				v, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				o.Set(k, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return o, nil
		case '[':
			arr := []any{}
			for dec.More() {
				v, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return arr, nil
		}
		return nil, fmt.Errorf("unexpected %v", t)
	case json.Number:
		return Number(t), nil
	default:
		return t, nil // string, bool, nil
	}
}

// Dump encodes v as Python's json.dumps(v, indent=2, ensure_ascii=False).
// No trailing newline.
func Dump(v any) []byte {
	var b strings.Builder
	dump(&b, v, 0)
	return []byte(b.String())
}

func dump(b *strings.Builder, v any, level int) {
	switch t := v.(type) {
	case *Object:
		if t.Len() == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteString("{")
		for i, k := range t.keys {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString("\n" + strings.Repeat("  ", level+1))
			writeString(b, k)
			b.WriteString(": ")
			dump(b, t.vals[k], level+1)
		}
		b.WriteString("\n" + strings.Repeat("  ", level) + "}")
	case []any:
		if len(t) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("[")
		for i, e := range t {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString("\n" + strings.Repeat("  ", level+1))
			dump(b, e, level+1)
		}
		b.WriteString("\n" + strings.Repeat("  ", level) + "]")
	case string:
		writeString(b, t)
	case Number:
		b.WriteString(pyNumber(string(t)))
	case bool:
		if t {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case nil:
		b.WriteString("null")
	default:
		fmt.Fprintf(b, "%v", t)
	}
}

// writeString escapes only what Python's ensure_ascii=False escapes: quote,
// backslash and control characters below 0x20.
func writeString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

// pyNumber re-prints a JSON number the way Python does after loading it:
// integers stay as written (bar "-0"), floats go through float.__repr__.
func pyNumber(lit string) string {
	if !strings.ContainsAny(lit, ".eE") {
		if lit == "-0" {
			return "0"
		}
		return lit
	}
	f, err := strconv.ParseFloat(lit, 64)
	if err != nil && !math.IsInf(f, 0) {
		return lit
	}
	if math.IsInf(f, 1) {
		return "Infinity"
	}
	if math.IsInf(f, -1) {
		return "-Infinity"
	}
	return floatRepr(f)
}

// floatRepr is Python's repr(float): shortest round-trip digits, fixed
// notation unless the exponent is below -4 or at least 16.
func floatRepr(f float64) string {
	sign := ""
	if math.Signbit(f) {
		sign = "-"
		f = -f
	}
	e := strconv.FormatFloat(f, 'e', -1, 64) // d.ddde±XX
	mant, expStr, _ := strings.Cut(e, "e")
	digits := strings.Replace(mant, ".", "", 1)
	exp, _ := strconv.Atoi(expStr)
	decpt := exp + 1
	if decpt <= -4 || decpt > 16 {
		m := digits[:1]
		if len(digits) > 1 {
			m += "." + digits[1:]
		}
		x := decpt - 1
		xs := "+"
		if x < 0 {
			xs = "-"
			x = -x
		}
		return fmt.Sprintf("%s%se%s%02d", sign, m, xs, x)
	}
	switch {
	case decpt <= 0:
		return sign + "0." + strings.Repeat("0", -decpt) + digits
	case decpt >= len(digits):
		return sign + digits + strings.Repeat("0", decpt-len(digits)) + ".0"
	default:
		return sign + digits[:decpt] + "." + digits[decpt:]
	}
}
