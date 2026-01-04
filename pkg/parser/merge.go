package parser

import (
	"container/heap"
	"context"
	"io"
)

// MergedSource combines multiple LogSources into a single stream
// ordered by timestamp (oldest first). This enables cross-service
// correlation by providing a unified timeline.
type MergedSource struct {
	sources []LogSource
	heap    *lineHeap
	closed  bool
}

// NewMergedSource creates a LogSource that merges multiple sources by timestamp.
// Lines are returned in chronological order across all sources.
func NewMergedSource(sources ...LogSource) *MergedSource {
	return &MergedSource{
		sources: sources,
		heap:    &lineHeap{},
	}
}

// Next returns the next log line in timestamp order across all sources.
// Returns io.EOF when all sources are exhausted.
func (m *MergedSource) Next(ctx context.Context) (*ParsedLine, error) {
	// Initialize heap on first call
	if m.heap.Len() == 0 && !m.closed {
		if err := m.initHeap(ctx); err != nil {
			return nil, err
		}
	}

	if m.heap.Len() == 0 {
		return nil, io.EOF
	}

	// Pop the oldest line
	item := heap.Pop(m.heap).(*heapItem)
	line := item.line

	// Refill from the same source
	if nextLine, err := m.sources[item.sourceIdx].Next(ctx); err == nil {
		heap.Push(m.heap, &heapItem{
			line:      nextLine,
			sourceIdx: item.sourceIdx,
		})
	} else if err != io.EOF {
		return nil, err
	}

	return line, nil
}

// initHeap reads the first line from each source to initialize the heap.
func (m *MergedSource) initHeap(ctx context.Context) error {
	heap.Init(m.heap)

	for i, src := range m.sources {
		line, err := src.Next(ctx)
		if err == io.EOF {
			continue // Empty source
		}
		if err != nil {
			return err
		}

		heap.Push(m.heap, &heapItem{
			line:      line,
			sourceIdx: i,
		})
	}

	return nil
}

// Close releases all source resources.
func (m *MergedSource) Close() error {
	m.closed = true
	var firstErr error
	for _, src := range m.sources {
		if err := src.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// heapItem wraps a ParsedLine with its source index for the priority queue.
type heapItem struct {
	line      *ParsedLine
	sourceIdx int
}

// lineHeap implements heap.Interface for timestamp-ordered merging.
type lineHeap []*heapItem

func (h lineHeap) Len() int { return len(h) }

func (h lineHeap) Less(i, j int) bool {
	return h[i].line.Timestamp.Before(h[j].line.Timestamp)
}

func (h lineHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *lineHeap) Push(x interface{}) {
	*h = append(*h, x.(*heapItem))
}

func (h *lineHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}
