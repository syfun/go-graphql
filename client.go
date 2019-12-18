package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/mitchellh/mapstructure"
)

// JSON represents json type.
type JSON map[string]interface{}

// Request represents graphql request body.
type Request struct {
	OperationName string `json:"operationName"`
	Query         string `json:"query"`
	Variable      JSON   `json:"variables"`
}

type SourceLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type GraphQLError struct {
	Message    string            `json:"message"`
	Path       []string          `json:"path"`
	Locations  []*SourceLocation `json:"locations"`
	Extensions JSON              `json:"extensions"`
}

type Response struct {
	Data   JSON            `json:"data"`
	Errors []*GraphQLError `json:"errors"`
}

func (r *Response) Guess(name string, v interface{}) error {
	if r.Data == nil {
		return errors.New("guess error: has no data")
	}

	d, ok := r.Data[name]
	if !ok {
		return fmt.Errorf("guess error: has no data about %v", name)
	}
	if err := mapstructure.Decode(d, v); err != nil {
		return fmt.Errorf("guess error: %w", err)
	}
	return nil
}

// Client represent graphql client, which can do query and mutatation.
type Client struct {
	url string

	// httpClient do the lower http request.
	httpClient *http.Client
}

// New create a graphql client with url and http client.
func New(url string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{url: url, httpClient: httpClient}
}

func (c *Client) buildHttpRequest(ctx context.Context, req *Request) (*http.Request, error) {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(req); err != nil {
		return nil, fmt.Errorf("build http request error: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.url, &body)
	if err != nil {
		return nil, fmt.Errorf("build http request error: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}

// Do exec graphql query or mutation.
func (c *Client) Do(ctx context.Context, req *Request) (*Response, error) {
	httpReq, err := c.buildHttpRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("graphql do error: %w", err)
	}
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		// If we got an error, and the context has been canceled,
		// the context's error is probably more useful.
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("graphql do error: %w", ctx.Err())
		default:
		}
		return nil, fmt.Errorf("graphql do error: %w", err)
	}
	defer httpResp.Body.Close()

	var resp Response
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("graphql do error: %w", err)
	}
	return &resp, nil
}
