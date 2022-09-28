package ksrpc

import "time"

type Handler struct {
	v int
}

type Stats struct {
	Calls int
	Val   int
	Last  string
}

func (h *Handler) Add(n int, _ *struct{}) error {
	h.v += n
	return nil
}

func (h *Handler) Stats(_ struct{}, s *Stats) error {
	*s = Stats{
		Calls: 10,
		Val:   h.v,
		Last:  time.Now().Format("01:02:03"),
	}
	return nil
}

func (h *Handler) Val(_ struct{}, i *int) {
	*i = h.v
}
