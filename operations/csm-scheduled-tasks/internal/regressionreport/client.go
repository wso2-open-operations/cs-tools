// Package regressionreport is the read-only client for entity-service's
// monthly product regression report (GET /reports/product-regressions) and
// the types it returns. internal/regressionscan turns the result into the
// mail; internal/notify renders it. The types live here, apart from the
// task, so notify can depend on them without the task depending on notify
// in a cycle -- the same split internal/migrationreport uses.
package regressionreport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/httpsec"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

var tokenFetchTimeout = 10 * time.Second

// maxResponseBytes bounds the report's body. A month's regressions for one
// business unit is a handful of rows; eight megabytes is a generous ceiling.
const maxResponseBytes = 8 << 20

// Report is one monthly product regression report as entity-service
// returns it.
type Report struct {
	GeneratedAt time.Time `json:"generatedAt"`
	// Month is the calendar month reported, "YYYY-MM".
	Month       string    `json:"month"`
	PeriodStart time.Time `json:"periodStart"`
	PeriodEnd   time.Time `json:"periodEnd"`
	Tag         string    `json:"tag"`
	Products    []string  `json:"products"`
	TotalCases  int       `json:"totalCases"`
	// Groups are the cases by product, in product name order.
	Groups []Group `json:"groups"`
}

// Group is one product's regression cases.
type Group struct {
	Product string `json:"product"`
	Cases   []Row  `json:"cases"`
}

// Row is one regression case.
type Row struct {
	ID              string    `json:"id"`
	Number          string    `json:"number"`
	IssueKey        string    `json:"issueKey"`
	Subject         string    `json:"subject"`
	Product         string    `json:"product"`
	DeployedProduct string    `json:"deployedProduct"`
	EtaSharedOn     time.Time `json:"etaSharedOn"`
}

// Config is the entity-service connection: the same OAuth client
// credentials every other entity-service client in this component uses.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client fetches the report.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client. Both URLs must be https, as everywhere
// else in this component.
func NewClient(cfg Config) (*Client, error) {
	if err := httpsec.RequireSecureURL(cfg.TokenURL); err != nil {
		return nil, fmt.Errorf("regressionreport: token URL: %w", err)
	}
	if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("regressionreport: base URL: %w", err)
	}

	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}

	tokenHTTPClient := &http.Client{Timeout: tokenFetchTimeout}
	httpsec.RejectInsecureRedirects(tokenHTTPClient)
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient, tokenHTTPClient)
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = 60 * time.Second
	httpsec.RejectInsecureRedirects(httpClient)

	return &Client{http: httpClient, baseURL: strings.TrimRight(cfg.BaseURL, "/")}, nil
}

// Report fetches last month's report for the named products.
// entity-service decides which month "last month" is, in the report's own
// timezone, from the instant of the call.
func (c *Client) Report(ctx context.Context, products []string) (Report, error) {
	q := url.Values{}
	q.Set("products", strings.Join(products, ","))

	endpoint := c.baseURL + "/reports/product-regressions?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Report{}, fmt.Errorf("regressionreport: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Report{}, fmt.Errorf("regressionreport: fetch report: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return Report{}, fmt.Errorf("regressionreport: read report: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Report{}, &apierror.Error{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var report Report
	if err := json.Unmarshal(body, &report); err != nil {
		return Report{}, fmt.Errorf("regressionreport: decode report: %w", err)
	}
	return report, nil
}
