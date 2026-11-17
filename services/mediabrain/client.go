// Package mediabrain talks to the Media Brain GraphQL API (https://brain.lan.bcc.media).
//
// Brain sits behind the authentik forward-auth proxy. Calls carry a token from the
// OAuth2 client_credentials grant of an authentik service account: the proxy accepts
// it, and passes it on to Brain, which signs the account in from its claims.
package mediabrain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-resty/resty/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/bcc-code/bcc-media-flows/internal/httpx"
)

const serviceName = "media brain"

// Sentinels for the codes Brain puts in a GraphQL error's extensions.code.
var (
	ErrUnauthenticated = errors.New("media brain: unauthenticated")
	ErrForbidden       = errors.New("media brain: forbidden")
	ErrNotFound        = errors.New("media brain: not found")
	ErrInvalidInput    = errors.New("media brain: invalid input")
	ErrConflict        = errors.New("media brain: conflict")
)

var errorCodes = map[string]error{
	"UNAUTHENTICATED": ErrUnauthenticated,
	"FORBIDDEN":       ErrForbidden,
	"NOT_FOUND":       ErrNotFound,
	"INVALID_INPUT":   ErrInvalidInput,
	"CONFLICT":        ErrConflict,
}

type Config interface {
	BaseURL() string
	TokenURL() string
	ClientID() string
	Username() string
	Password() string
}

type Client struct {
	client *resty.Client
}

// tokenTimeout bounds one call to the token endpoint; the oauth2 package has no
// timeout of its own.
const tokenTimeout = 30 * time.Second

func NewClient(cfg Config) *Client {
	cc := clientcredentials.Config{
		ClientID:  cfg.ClientID(),
		TokenURL:  cfg.TokenURL(),
		Scopes:    []string{"openid", "profile"},
		AuthStyle: oauth2.AuthStyleInParams,
		// authentik takes the service account and its app password as extra params
		// of the client_credentials grant.
		EndpointParams: url.Values{
			"username": {cfg.Username()},
			"password": {cfg.Password()},
		},
	}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Timeout: tokenTimeout})

	// The token source caches the token and fetches a new one when it expires. There
	// is no refresh token in this grant.
	return newClient(cfg.BaseURL(), cc.TokenSource(ctx))
}

func newClient(baseURL string, tokens oauth2.TokenSource) *Client {
	client := httpx.New(httpx.Config{
		Service:       serviceName,
		BaseURL:       baseURL,
		DescribeError: describeError,
	})

	client.OnBeforeRequest(func(_ *resty.Client, req *resty.Request) error {
		token, err := tokens.Token()
		if err != nil {
			return fmt.Errorf("getting Media Brain token: %w", err)
		}
		req.SetAuthToken(token.AccessToken)
		return nil
	})

	return &Client{client: client}
}

// GraphQLError is one entry of a GraphQL response's errors. It unwraps to the
// sentinel for its code, if Brain sent a known one.
type GraphQLError struct {
	Message    string `json:"message"`
	Path       []any  `json:"path,omitempty"`
	Extensions struct {
		Code string `json:"code"`
	} `json:"extensions"`
}

func (e *GraphQLError) Error() string {
	if e.Extensions.Code == "" {
		return "media brain: " + e.Message
	}
	return fmt.Sprintf("media brain: %s (%s)", e.Message, e.Extensions.Code)
}

func (e *GraphQLError) Unwrap() error { return errorCodes[e.Extensions.Code] }

type graphQLRequest struct {
	Query     string `json:"query"`
	Variables any    `json:"variables,omitempty"`
}

type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []GraphQLError  `json:"errors"`
}

func joinErrors(list []GraphQLError) error {
	errs := make([]error, len(list))
	for i := range list {
		errs[i] = &list[i]
	}
	return errors.Join(errs...)
}

// describeError keeps the GraphQL errors of a non-2xx response, which is how Brain
// refuses a request before it reaches the resolvers (401, 403).
func describeError(resp *resty.Response) error {
	var body graphQLResponse
	if err := json.Unmarshal(resp.Body(), &body); err != nil || len(body.Errors) == 0 {
		return httpx.Describe(serviceName, resp)
	}
	err := joinErrors(body.Errors)
	return &httpx.StatusError{
		StatusCode: resp.StatusCode(),
		Message:    fmt.Sprintf("%s (status %d)", err, resp.StatusCode()),
		Err:        err,
	}
}

// do runs one GraphQL operation and decodes its data into out.
func (c *Client) do(ctx context.Context, query string, variables any, out any) error {
	var body graphQLResponse
	_, err := c.client.R().
		SetContext(ctx).
		SetBody(graphQLRequest{Query: query, Variables: variables}).
		SetResult(&body).
		Post("/query")
	if err != nil {
		return httpx.SanitizeError(err)
	}
	if len(body.Errors) > 0 {
		return joinErrors(body.Errors)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body.Data, out); err != nil {
		return fmt.Errorf("decoding Media Brain response: %w", err)
	}
	return nil
}
