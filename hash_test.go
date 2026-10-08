package kitsync

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"
)

func TestHashBytesEmpty(t *testing.T) {
	got := HashBytes([]byte{})
	want := sha256.New()
	hex.EncodeToString(want.Sum(nil))
	// Actually compute the expected hash of empty slice.
	empty := sha256.Sum256([]byte{})
	wantHex := hex.EncodeToString(empty[:])
	if got != wantHex {
		t.Errorf("HashBytes([]) = %q, want %q", got, wantHex)
	}
}

func TestHashBytesKnownInput(t *testing.T) {
	input := []byte("hello")
	got := HashBytes(input)

	h := sha256.New()
	h.Write(input)
	want := hex.EncodeToString(h.Sum(nil))
	if got != want {
		t.Errorf("HashBytes(%q) = %q, want %q", input, got, want)
	}
}

func TestHashBytesDeterministic(t *testing.T) {
	input := []byte("deterministic test")
	got1 := HashBytes(input)
	got2 := HashBytes(input)
	if got1 != got2 {
		t.Errorf("HashBytes not deterministic: %q != %q", got1, got2)
	}
}

func TestHashBytesDifferentInputs(t *testing.T) {
	h1 := HashBytes([]byte("a"))
	h2 := HashBytes([]byte("b"))
	if h1 == h2 {
		t.Error("HashBytes produced same hash for different inputs")
	}
}

func TestHashReaderEmpty(t *testing.T) {
	r := bytes.NewReader([]byte{})
	got, err := HashReader(r)
	if err != nil {
		t.Fatalf("HashReader(empty): %v", err)
	}
	empty := sha256.Sum256([]byte{})
	want := hex.EncodeToString(empty[:])
	if got != want {
		t.Errorf("HashReader(empty) = %q, want %q", got, want)
	}
}

func TestHashReaderKnownInput(t *testing.T) {
	input := []byte("hello reader")
	r := bytes.NewReader(input)
	got, err := HashReader(r)
	if err != nil {
		t.Fatalf("HashReader: %v", err)
	}

	h := sha256.New()
	io.Copy(h, bytes.NewReader(input))
	want := hex.EncodeToString(h.Sum(nil))
	if got != want {
		t.Errorf("HashReader(%q) = %q, want %q", input, got, want)
	}
}

func TestHashReaderDeterministic(t *testing.T) {
	input := []byte("deterministic reader test")
	r1 := bytes.NewReader(input)
	got1, err := HashReader(r1)
	if err != nil {
		t.Fatalf("HashReader first: %v", err)
	}

	r2 := bytes.NewReader(input)
	got2, err := HashReader(r2)
	if err != nil {
		t.Fatalf("HashReader second: %v", err)
	}
	if got1 != got2 {
		t.Errorf("HashReader not deterministic: %q != %q", got1, got2)
	}
}

func TestHashBytesAndHashReaderMatch(t *testing.T) {
	input := []byte("same input for both")
	gotBytes := HashBytes(input)
	gotReader, err := HashReader(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("HashReader: %v", err)
	}
	if gotBytes != gotReader {
		t.Errorf("HashBytes(%q) = %q, but HashReader(%q) = %q; should match", input, gotBytes, input, gotReader)
	}
}
