package platform

import "testing"

func TestVectorRoundTrip(t *testing.T) {
	in := NewVector([]float32{0.25, -1, 3.5e-7})
	v, err := in.Value()
	if err != nil {
		t.Fatal(err)
	}
	var out Vector
	if err := out.Scan(v); err != nil {
		t.Fatal(err)
	}
	if !out.Equal(in) {
		t.Errorf("round trip: %v != %v", out.Floats(), in.Floats())
	}
	var empty Vector
	if v, _ := empty.Value(); v != nil {
		t.Errorf("zero vector stores %v, want NULL", v)
	}
	if err := out.Scan([]byte("not a vector")); err == nil {
		t.Error("malformed text accepted")
	}
}

func TestCosine(t *testing.T) {
	a, b := NewVector([]float32{1, 0}), NewVector([]float32{0, 1})
	if a.Cosine(a) < 0.999 || a.Cosine(b) != 0 {
		t.Errorf("cosine: %v %v", a.Cosine(a), a.Cosine(b))
	}
}
