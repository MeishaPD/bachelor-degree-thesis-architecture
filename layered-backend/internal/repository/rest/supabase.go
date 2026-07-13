package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxErrorBodyBytes = 4096

type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
	apiKey     string
}

func NewClient(httpClient *http.Client, baseURL string, apiKey string) (*Client, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("http client tidak boleh nil")
	}

	baseURL = strings.TrimSpace(baseURL)
	apiKey = strings.TrimSpace(apiKey)

	if baseURL == "" {
		return nil, fmt.Errorf("SUPABASE_URL wajib diisi")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("SUPABASE_API_KEY wajib diisi")
	}

	parsedURL, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse SUPABASE_URL: %w", err)
	}

	if parsedURL.Scheme != "https" || parsedURL.Host == "" {
		return nil, fmt.Errorf("SUPABASE_URL harus berupa URL HTTPS yang valid")
	}

	return &Client{
		httpClient: httpClient,
		baseURL:    parsedURL,
		apiKey:     apiKey,
	}, nil
}

func (c *Client) DoJSON(ctx context.Context, method string, resource string, query url.Values, body any, headers map[string]string, out any) error {
	requestURL := *c.baseURL
	requestURL.Path = strings.TrimRight(requestURL.Path, "/") + "/rest/v1/" + strings.TrimLeft(resource, "/")
	requestURL.RawQuery = query.Encode()

	var requestBody io.Reader

	if body != nil {
		encodedBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request Supabase: %w", err)
		}
		requestBody = bytes.NewReader(encodedBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL.String(), requestBody)
	if err != nil {
		return fmt.Errorf("buat request Supabase: %w", err)
	}

	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Accept", "application/json")

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("akses Supabase Data API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		if readErr != nil {
			return fmt.Errorf("Supabase return %s "+"& body: %w", resp.Status, readErr)
		}

		return fmt.Errorf("Supabase return %s: %s", resp.Status, strings.TrimSpace(string(responseBody)))
	}

	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode respons Supabase: %w", err)
	}

	return nil
}
