// Package clients contains the HTTP adapters that talk to the downstream
// services (pdf-validator, pdf-persistence, pdf-converter). Each client wraps
// its calls in a circuit breaker: transport errors, timeouts and 5xx trip the
// breaker and surface as orchestrator.ErrDownstream; 4xx business errors are
// returned untouched so callers can map them to domain errors without tripping
// the breaker.
package clients

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
	"github.com/sony/gobreaker"
)

const requestTimeout = 30 * time.Second

// httpClient performs HTTP requests against one downstream service through a
// circuit breaker. It is shared by the three concrete clients.
type httpClient struct {
	name    string
	baseURL string
	client  *http.Client
	breaker *gobreaker.CircuitBreaker
}

func newHTTPClient(name, baseURL string) *httpClient {
	return &httpClient{
		name:    name,
		baseURL: baseURL,
		client:  &http.Client{Timeout: requestTimeout},
		breaker: gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    name,
			Timeout: 30 * time.Second,
		}),
	}
}

// httpResult carries a non-failure response (status < 500) out of the breaker.
type httpResult struct {
	status int
	body   []byte
}

// do performs a request and returns the response status and body. It returns
// orchestrator.ErrDownstream for transport errors, timeouts, 5xx responses and
// when the breaker is open. 4xx responses are returned as results (not
// errors) so they never count against the breaker.
func (c *httpClient) do(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
	result, err := c.breaker.Execute(func() (any, error) {
		var r io.Reader
		if body != nil {
			r = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, r)
		if err != nil {
			return nil, err
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 500 {
			return nil, fmt.Errorf("%s: downstream returned %d", c.name, resp.StatusCode)
		}
		return &httpResult{status: resp.StatusCode, body: data}, nil
	})
	if err != nil {
		return 0, nil, orchestrator.ErrDownstream
	}
	res := result.(*httpResult)
	return res.status, res.body, nil
}
