package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/msjurset/runbook/internal/runbook"
)

// HTTPExecutor makes an HTTP request.
type HTTPExecutor struct {
	Step   *runbook.HTTPStep
	Client *http.Client // nil uses http.DefaultClient
}

func (e *HTTPExecutor) Execute(ctx context.Context, vars map[string]string, stdout, stderr io.Writer) (*ExecResult, error) {
	method := e.Step.Method
	if method == "" {
		method = "GET"
	}

	url, err := runbook.Expand(e.Step.URL, vars)
	if err != nil {
		return nil, fmt.Errorf("expanding url: %w", err)
	}

	var body io.Reader
	if e.Step.Body != "" {
		expanded, err := runbook.Expand(e.Step.Body, vars)
		if err != nil {
			return nil, fmt.Errorf("expanding body: %w", err)
		}
		body = strings.NewReader(expanded)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	for k, v := range e.Step.Headers {
		expanded, err := runbook.Expand(v, vars)
		if err != nil {
			return nil, fmt.Errorf("expanding header %q: %w", k, err)
		}
		req.Header.Set(k, expanded)
	}

	client := e.Client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	bodyStr := string(respBody)

	// Stream the response to stdout
	if stdout != nil {
		fmt.Fprintf(stdout, "HTTP %d %s\n", resp.StatusCode, resp.Status)
		if len(respBody) > 0 {
			stdout.Write(respBody)
			if !bytes.HasSuffix(respBody, []byte("\n")) {
				stdout.Write([]byte("\n"))
			}
		}
	}

	result := &ExecResult{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       bodyStr,
	}

	// Treat 4xx and 5xx as errors
	if resp.StatusCode >= 400 {
		return result, fmt.Errorf("http %d: %s", resp.StatusCode, resp.Status)
	}

	return result, nil
}
