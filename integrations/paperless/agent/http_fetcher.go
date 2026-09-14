package agent

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HTTPFetcher struct {
	BaseURL    string
	Token      string
	APIVersion string
	Client     *http.Client
}

var (
	ErrInvalidConfig     = errors.New("invalid paperless http fetcher configuration")
	ErrFetchUnauthorized = errors.New("paperless fetch was unauthorized")
	ErrFetchNotFound     = errors.New("paperless document was not found")
	ErrFetchFailed       = errors.New("paperless fetch failed")
)

// Fetch retrieves the selected Paperless representation through the documented
// document download endpoint. The caller owns closing the returned document.
func (f HTTPFetcher) Fetch(req FetchRequest) (FetchResult, error) {
	if !validText(req.Instance) || !validText(req.DocumentID) || !validText(req.Version) || f.Token == "" {
		return FetchResult{}, ErrInvalidConfig
	}
	u, err := f.downloadURL(req)
	if err != nil {
		return FetchResult{}, err
	}
	httpReq, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return FetchResult{}, err
	}
	httpReq.Header.Set("Authorization", "Token "+f.Token)
	httpReq.Header.Set("Accept", f.acceptHeader())
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return FetchResult{}, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return FetchResult{}, fmt.Errorf("%w: status %d", ErrFetchUnauthorized, resp.StatusCode)
		case http.StatusNotFound:
			return FetchResult{}, fmt.Errorf("%w: status %d", ErrFetchNotFound, resp.StatusCode)
		default:
			return FetchResult{}, fmt.Errorf("%w: status %d", ErrFetchFailed, resp.StatusCode)
		}
	}
	return FetchResult{Version: req.Version, Document: resp.Body}, nil
}

func (f HTTPFetcher) downloadURL(req FetchRequest) (string, error) {
	if !validText(f.BaseURL) {
		return "", ErrInvalidConfig
	}
	base, err := url.Parse(f.BaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", ErrInvalidConfig
	}
	u := base.JoinPath("api", "documents", req.DocumentID, "download")
	q := u.Query()
	q.Set("version", req.Version)
	if req.Representation == OriginalUpload {
		q.Set("original", "true")
	} else if req.Representation != ArchiveOutput {
		return "", ErrInvalidConfig
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (f HTTPFetcher) acceptHeader() string {
	version := strings.TrimSpace(f.APIVersion)
	if version == "" {
		version = "10"
	}
	return "application/json; version=" + version + ", */*;q=0.1"
}
