package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/mitchellh/mapstructure"
)

// JSON represents json type.
type JSON map[string]interface{}

// NamedReader is the interface that groups the basic Read methods and a Name method.
type NamedReader interface {
	io.Reader
	Name() string
}

// Request represents graphql request body.
type Request struct {
	OperationName string `json:"operationName"`
	Query         string `json:"query"`
	Variables     JSON   `json:"variables"`

	Files []NamedReader
}

// NewRequest build new graphql request with operation name, query or mutation, and variables.
func NewRequest(query, operationName string, variables JSON) *Request {
	return &Request{
		OperationName: operationName,
		Query:         query,
		Variables:     variables,
	}
}

// NewUploadRequest build a new single upload request.
func NewUploadRequest(query, operationName string, file ...NamedReader) *Request {
	return &Request{
		OperationName: operationName,
		Query:         query,
		Files:         file,
	}
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
	if r.HasError() {
		return r
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

// Copy a new graphql client with a http client.
func (c *Client) Copy(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{url: c.url, httpClient: httpClient}
}

func (c *Client) buildJSONRequest(ctx context.Context, req *Request) (*http.Request, error) {
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

func (c *Client) do(ctx context.Context, req *Request, httpReq *http.Request) (*Response, error) {
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

	resp := &Response{req: req}
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("graphql do error: %w", err)
	}
	if resp.HasError() {
		return nil, resp
	}
	return resp, nil
}

// Do exec graphql query or mutation.
func (c *Client) Do(ctx context.Context, query, operationName string, variables JSON) (*Response, error) {
	req := NewRequest(query, operationName, variables)
	httpReq, err := c.buildJSONRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("graphql do error: %w", err)
	}
	return c.do(ctx, req, httpReq)
}

func writeField(w *multipart.Writer, fieldname string, value interface{}) error {
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("write field %v error: %w", fieldname, err)
	}
	if err := w.WriteField(fieldname, string(b)); err != nil {
		return fmt.Errorf("write field %v error: %w", fieldname, err)
	}
	return nil
}

func writeFile(w *multipart.Writer, fieldname string, file NamedReader) error {
	f, err := w.CreateFormFile(fieldname, file.Name())
	if err != nil {
		return fmt.Errorf("write file %v error: %w", fieldname, err)
	}
	if _, err := io.Copy(f, file); err != nil {
		return fmt.Errorf("write file %v error: %w", fieldname, err)
	}
	return nil
}

func (c *Client) buildFormDataRequest(ctx context.Context, req *Request, single bool) (*http.Request, error) {
	if req.Files == nil || len(req.Files) == 0 {
		return nil, errors.New("build form data request error: has no files")
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	var files []NamedReader
	m := make(JSON)
	if single {
		files = req.Files[:1]
		req.Variables = JSON{"file": nil}
		m["0"] = []string{"variables.file"}
	} else {
		files = req.Files
		s := []*struct{}{}
		for i := range files {
			m[strconv.Itoa(i)] = []string{fmt.Sprintf("variables.files.%v", i)}
			s = append(s, nil)
		}
		req.Variables = JSON{"files": s}
	}
	if err := writeField(w, "operations", req); err != nil {
		return nil, fmt.Errorf("build form data request error: %w", err)
	}
	if err := writeField(w, "map", m); err != nil {
		return nil, fmt.Errorf("build form data request error: %w", err)
	}
	for i, file := range files {
		if err := writeFile(w, strconv.Itoa(i), file); err != nil {
			return nil, fmt.Errorf("build form data request error: %w", err)
		}
	}
	w.Close()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.url, &body)
	if err != nil {
		return nil, fmt.Errorf("build form data request error: %w", err)
	}
	httpReq.Header.Set("Content-Type", w.FormDataContentType())
	return httpReq, nil
}

// SingleUpload implement [GraphQL multipart request specification](https://github.com/jaydenseric/graphql-multipart-request-spec)
func (c *Client) SingleUpload(ctx context.Context, query, operationName string, file NamedReader) (*Response, error) {
	req := NewUploadRequest(query, operationName, file)
	httpReq, err := c.buildFormDataRequest(ctx, req, true)
	if err != nil {
		return nil, fmt.Errorf("graphql single upload error: %w", err)
	}
	return c.do(ctx, req, httpReq)
}

// MultiUpload implement [GraphQL multipart request specification](https://github.com/jaydenseric/graphql-multipart-request-spec)
func (c *Client) MultiUpload(ctx context.Context, query, operationName string, file ...NamedReader) (*Response, error) {
	req := NewUploadRequest(query, operationName, file...)
	httpReq, err := c.buildFormDataRequest(ctx, req, false)
	if err != nil {
		return nil, fmt.Errorf("graphql single upload error: %w", err)
	}
	return c.do(ctx, req, httpReq)
}
