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
		client: &http.Client{
			Timeout: requestTimeout,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 100, // o al menos vus_max de tu benchmark
			},
		},
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
// a *orchestrator.DownstreamError for transport errors, timeouts, 5xx responses
// and when the breaker is open; the error names the service and carries the
// status when known. 4xx responses are returned as results (not errors) so
// they never count against the breaker.
func (c *httpClient) do(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
	var status int
	result, err := c.breaker.Execute(func() (any, error) {
		status = 0
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
		status = resp.StatusCode
		if resp.StatusCode >= 500 {
			return nil, fmt.Errorf("downstream returned %d", resp.StatusCode)
		}
		return &httpResult{status: resp.StatusCode, body: data}, nil
	})
	if err != nil {
		return 0, nil, c.downstreamError(status, err)
	}
	res := result.(*httpResult)
	return res.status, res.body, nil
}

// downstreamError builds a typed ErrDownstream carrying the service name,
// status and underlying cause.
func (c *httpClient) downstreamError(status int, cause error) error {
	if cause == nil {
		cause = orchestrator.ErrDownstream
	}
	return orchestrator.DownstreamError{Service: c.name, Status: status, Err: cause}
}
