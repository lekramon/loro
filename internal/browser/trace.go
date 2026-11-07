package browser

import (
	"context"
	"encoding/json"
	"github.com/lekramon/piratarr/internal/communications"
)

// Only this small JSON protocol is accepted from stderr; library/native output
// and unknown fields are discarded instead of becoming public diagnostics.
type traceWriter struct {
	ctx       context.Context
	operation string
	line      []byte
	dropping  bool
}

func (w *traceWriter) Write(data []byte) (int, error) {
	for _, b := range data {
		if b == '\n' {
			if !w.dropping {
				w.consume()
			}
			w.line = w.line[:0]
			w.dropping = false
			continue
		}
		if !w.dropping {
			if len(w.line) >= 4096 {
				w.dropping = true
				w.line = w.line[:0]
			} else {
				w.line = append(w.line, b)
			}
		}
	}
	return len(data), nil
}
func (w *traceWriter) consume() {
	var packet struct {
		Phase     string `json:"phase"`
		Status    *int   `json:"status"`
		Count     *int64 `json:"count"`
		ElapsedMS *int64 `json:"elapsedMs"`
		Code      string `json:"code"`
		Kind      string `json:"kind"`
	}
	if json.Unmarshal(w.line, &packet) != nil || !communications.ValidPhase(packet.Phase) {
		return
	}
	operation := w.operation
	switch packet.Kind {
	case "document", "script", "stylesheet", "image", "xhr", "fetch", "font", "media", "other":
		operation += "." + packet.Kind
	}
	event := communications.Event{Integration: "browser", Operation: operation, Phase: packet.Phase, Status: packet.Status, Count: packet.Count, ElapsedMS: packet.ElapsedMS}
	switch packet.Code {
	case "source-browser-unavailable", "source-unavailable", "source-timeout", "source-challenge-required", "source-invalid-response", "source-rate-limited", "missing-reference", "ambiguous-reference", "unsupported-reference":
		event.Code = &packet.Code
		event.Level = "warning"
	}
	communications.Emit(w.ctx, event)
}
