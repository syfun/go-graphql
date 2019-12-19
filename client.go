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

// SourceLocation represents a location in a Source.
type SourceLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// GraphQLError describes an Error found during the parse, validate, or execute
// phases of performing a GraphQL operation. In addition to a message, it also includes
// information about the locations in a GraphQL document and/or execution result that
// correspond to the Error.
type GraphQLError struct {
	Message    string            `json:"message"`
	Path       []string          `json:"path"`
	Locations  []*SourceLocation `json:"locations"`
	Extensions JSON              `json:"extensions"`
}

// Response represents graphql return value.
type Response struct {
	Data   JSON            `json:"data"`
	Errors []*GraphQLError `json:"errors"`
	req    *Request
}

// Guess with name and convert data to v.
// Example:
//
// If response data is
// {
//   "data": {
//	   "person": {
//	     "name": "Jack"
//		 "age": 26
// 	   }
//   },
// 	 "error": null
// }
//
// type Person struct {
//   name string
//   age int
// }
//
// var p Person
// r.Guess("person", p)
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

// HasError will check whether response has errors.
func (r *Response) HasError() bool {
	if r.Errors == nil || len(r.Errors) == 0 {
		return false
	}
	return true
}

// Error return response error string.
func (r *Response) Error() string {
	b, _ := json.MarshalIndent(r.Errors, "", "  ")
	return fmt.Sprintf("%v error: %v", r.req.OperationName, string(b))
}

// Client represent graphql client, which can do query and mutatation.
type Client struct {
	url string

	// HTTPClient do the lower http request.
	HTTPClient *http.Client
}

// New create a graphql client with url and http client.
func New(url string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{url: url, HTTPClient: httpClient}
}

func (c *Client) buildHTTPRequest(ctx context.Context, req *Request) (*http.Request, error) {
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
	httpReq, err := c.buildHTTPRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("graphql do error: %w", err)
	}
	httpResp, err := c.HTTPClient.Do(httpReq)
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

	resp := &Response{req: req}
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("graphql do error: %w", err)
	}
	if resp.HasError() {
		return nil, resp
	}
	return resp, nil
}
