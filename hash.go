package kitsync

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
)

func HashReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	s := h.Sum(nil)
	return hex.EncodeToString(s), nil
}

func HashBytes(bs []byte) string {
	h := sha256.New()
	h.Write(bs)
	s := h.Sum(nil)
	return hex.EncodeToString(s)
}
