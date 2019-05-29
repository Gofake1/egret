package main

import (
	"container/heap" // Init

	"github.com/emersion/go-imap" // Message
)

type MessageHeap []*imap.Message

func newMessageHeap() *MessageHeap {
	h := &MessageHeap{}
	heap.Init(h)
	return h
}

func (h MessageHeap) Len() int {
	return len(h)
}

func (h MessageHeap) Less(i, j int) bool {
	return h[i].Envelope.Date.After(h[j].Envelope.Date)
}

func (h MessageHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *MessageHeap) Push(x interface{}) {
	*h = append(*h, x.(*imap.Message))
}

func (h *MessageHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
