package shortcode

import "testing"

func TestEncodeDecodeRoundTrip(t *testing.T) {
	ids := []uint64{0, 1, 61, 62, 63, 125, 1000, 99999, 12345678, 1<<63 - 1}

	for _, id := range ids {
		code := Encode(id)
		decoded, err := Decode(code)
		if err != nil {
			t.Fatalf("Decode(%q) returned unexpected error: %v", code, err)
		}
		if decoded != id {
			t.Errorf("round trip mismatch: Encode(%d) = %q, Decode(%q) = %d", id, code, code, decoded)
		}
	}
}

func TestEncodeKnownValues(t *testing.T) {
	cases := map[uint64]string{
		0:   "0",
		1:   "1",
		61:  "z",
		62:  "10",
		63:  "11",
		125: "21",
	}

	for id, want := range cases {
		got := Encode(id)
		if got != want {
			t.Errorf("Encode(%d) = %q, want %q", id, got, want)
		}
	}
}

func TestDecodeInvalidCharacter(t *testing.T) {
	invalidInputs := []string{"", "abc+123", "has space", "слово", "-1"}

	for _, in := range invalidInputs {
		if _, err := Decode(in); err != ErrInvalidCharacter {
			t.Errorf("Decode(%q) error = %v, want ErrInvalidCharacter", in, err)
		}
	}
}

func TestEncodeIsDeterministic(t *testing.T) {
	for i := uint64(0); i < 1000; i++ {
		if Encode(i) != Encode(i) {
			t.Fatalf("Encode(%d) is not deterministic", i)
		}
	}
}

func TestEncodeMonotonicLength(t *testing.T) {
	// Base62 encoding should never produce a shorter code for a larger ID
	// within the same "digit class" (e.g. all 2-digit codes < all 3-digit codes
	// is not strictly true across the boundary, but length should never decrease
	// as id increases past a power-of-base boundary).
	prevLen := len(Encode(0))
	for i := uint64(1); i < 100000; i++ {
		l := len(Encode(i))
		if l < prevLen {
			t.Fatalf("Encode(%d) has length %d, shorter than previous length %d", i, l, prevLen)
		}
		prevLen = l
	}
}
