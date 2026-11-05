// Package eolusagereport is the read-only client for entity-service's
// monthly EOL product usage report (GET /reports/eol-product-usage) and the
// types it returns. internal/eolusagescan turns the result into the mail;
// internal/notify renders it. The types live here, apart from the task, so
// notify can depend on them without the task depending on notify in a
// cycle -- the same split internal/regressionreport uses.
package eolusagereport

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

// maxResponseBytes bounds the report's body. Every Primary Production
// deployment on an EOL version is a few hundred rows at most; eight megabytes
// is a generous ceiling.
const maxResponseBytes = 8 << 20

// Report is one EOL product usage report as entity-service returns it: the
// customers running a product version past its EOL date in an active
// Primary Production deployment, grouped by region, then account, then
// project.
type Report struct {
	GeneratedAt time.Time `json:"generatedAt"`
	// AsOf is the report date, "YYYY-MM-DD": a version is listed when its EOL
	// date is before it.
	AsOf string `json:"asOf"`
	// TotalVersions counts the version entries across every project.
	TotalVersions int `json:"totalVersions"`
	// TotalAccounts counts the accounts listed.
	TotalAccounts int `json:"totalAccounts"`
	// Regions are in name order; the accounts with no region ("") come last.
	Regions []Region `json:"regions"`
}

// Region is the accounts of one region.
type Region struct {
	// Region is the account's region, "" when it has none.
	Region   string    `json:"region"`
	Accounts []Account `json:"accounts"`
}

// Account is one customer account and its projects with EOL usage.
type Account struct {
	AccountID   string `json:"accountId"`
	AccountName string `json:"accountName"`
	// CsAbt is the account's CS (CRE) team name, "" when none.
	CsAbt  string `json:"csAbt"`
	Region string `json:"region"`
	// AccountManager and TechnicalOwner are nil when the account has none.
	AccountManager *Person   `json:"accountManager"`
	TechnicalOwner *Person   `json:"technicalOwner"`
	Projects       []Project `json:"projects"`
}

// Person is an account's manager or technical owner.
type Person struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Project is one project and the EOL versions in its Primary Production
// deployments.
type Project struct {
	ProjectID   string `json:"projectId"`
	ProjectName string `json:"projectName"`
	// ProjectKey is the project's key ("ACMEPROD"), "" when none.
	ProjectKey string    `json:"projectKey"`
	Versions   []Version `json:"versions"`
}

// Version is one product version past its EOL date.
type Version struct {
	ProductVersionID string `json:"productVersionId"`
	ProductName      string `json:"productName"`
	Version          string `json:"version"`
	// SupportStatus is the stored enum text, upper case ("EXTENDED"); nil
	// when the version has none.
	SupportStatus *string `json:"supportStatus"`
	// EOLDate is "YYYY-MM-DD".
	EOLDate string `json:"eolDate"`
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
		return nil, fmt.Errorf("eolusagereport: token URL: %w", err)
	}
	if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("eolusagereport: base URL: %w", err)
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

// Report fetches the report as of asOf ("YYYY-MM-DD"). The task passes its
// run date in the report's timezone, so a retry later the same day reports
// the same date.
func (c *Client) Report(ctx context.Context, asOf string) (Report, error) {
	q := url.Values{}
	q.Set("asOf", asOf)

	endpoint := c.baseURL + "/reports/eol-product-usage?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Report{}, fmt.Errorf("eolusagereport: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Report{}, fmt.Errorf("eolusagereport: fetch report: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return Report{}, fmt.Errorf("eolusagereport: read report: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Report{}, &apierror.Error{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var report Report
	if err := json.Unmarshal(body, &report); err != nil {
		return Report{}, fmt.Errorf("eolusagereport: decode report: %w", err)
	}
	return report, nil
}
