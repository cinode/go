package graph

type headWriter struct {
	limit int
	data  []byte
}

func newHeadWriter(limit int) headWriter {
	return headWriter{
		limit: limit,
		data:  make([]byte, limit),
	}
}

func (h *headWriter) Write(b []byte) (int, error) {
	if len(h.data) >= h.limit {
		return len(b), nil
	}

	if len(h.data)+len(b) > h.limit {
		h.data = append(h.data, b[:h.limit-len(h.data)]...)
		return len(b), nil
	}

	h.data = append(h.data, b...)
	return len(b), nil
}
