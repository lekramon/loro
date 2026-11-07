package communications

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"
)

type Transport struct {
	Base        http.RoundTripper
	Integration string
}

func (t Transport) RoundTrip(request *http.Request) (*http.Response, error) {
	ctx := Correlate(request.Context())
	request = request.WithContext(ctx)
	operation := t.Integration + ".request"
	Emit(ctx, Event{Integration: t.Integration, Operation: operation, Phase: "request.started"})
	start := time.Now()
	response, err := t.Base.RoundTrip(request)
	e := Event{Integration: t.Integration, Operation: operation, Phase: "response.received", ElapsedMS: Ptr(time.Since(start).Milliseconds())}
	if err != nil {
		e.Phase = "request.failed"
		e.Level = "error"
		code := "network-unavailable"
		var network net.Error
		if errors.Is(ctx.Err(), context.Canceled) {
			code = "request-cancelled"
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.As(err, &network) && network.Timeout() {
			code = "request-timeout"
		}
		e.Code = &code
	} else {
		e.Status = Ptr(response.StatusCode)
		if response.StatusCode >= 400 {
			e.Level = "warning"
		}
		if value := response.Header.Get("Retry-After"); value != "" {
			delay, err := strconv.Atoi(value)
			if err != nil {
				if stamp, parseErr := http.ParseTime(value); parseErr == nil {
					delay = int(time.Until(stamp).Seconds()) + 1
				}
			}
			if delay < 0 {
				delay = 0
			}
			if delay > 3600 {
				delay = 3600
			}
			e.RetryAfter = &delay
		}
	}
	Emit(ctx, e)
	return response, err
}
