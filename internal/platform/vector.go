package platform

import (
	"database/sql/driver"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

// Vector is a pgvector value. The zero Vector stores as NULL.
type Vector struct{ v []float32 }

func NewVector(f []float32) Vector { return Vector{v: slices.Clone(f)} }

func (x Vector) Floats() []float32   { return x.v }
func (x Vector) Dim() int            { return len(x.v) }
func (x Vector) IsZero() bool        { return len(x.v) == 0 }
func (x Vector) Equal(o Vector) bool { return slices.Equal(x.v, o.v) }

// Cosine returns the cosine similarity of two vectors of the same dimension.
func (x Vector) Cosine(o Vector) float64 {
	var dot, nx, no float64
	for i := range x.v {
		dot += float64(x.v[i]) * float64(o.v[i])
		nx += float64(x.v[i]) * float64(x.v[i])
		no += float64(o.v[i]) * float64(o.v[i])
	}
	if nx == 0 || no == 0 {
		return 0
	}
	return dot / (math.Sqrt(nx) * math.Sqrt(no))
}

func (x Vector) Value() (driver.Value, error) {
	if x.IsZero() {
		return nil, nil
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, f := range x.v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(f), 'g', -1, 32))
	}
	b.WriteByte(']')
	return b.String(), nil
}

func (x *Vector) Scan(src any) error {
	var s string
	switch t := src.(type) {
	case nil:
		x.v = nil
		return nil
	case string:
		s = t
	case []byte:
		s = string(t)
	default:
		return fmt.Errorf("vector: cannot scan %T", src)
	}
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return fmt.Errorf("vector: malformed value %.40q", s)
	}
	s = s[1 : len(s)-1]
	x.v = nil
	if s == "" {
		return nil
	}
	for p := range strings.SplitSeq(s, ",") {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return fmt.Errorf("vector: %w", err)
		}
		x.v = append(x.v, float32(f))
	}
	return nil
}
