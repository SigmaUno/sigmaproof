package agent

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type HTTPFetcher struct {
	BaseURL    string
	Token      string
	APIVersion string
	Client     *http.Client
	MaxBytes   int64
}

var (
	ErrInvalidConfig     = errors.New("invalid paperless http fetcher configuration")
	ErrFetchUnauthorized = errors.New("paperless fetch was unauthorized")
	ErrFetchNotFound     = errors.New("paperless document was not found")
	ErrFetchFailed       = errors.New("paperless fetch failed")
	ErrArchiveMissing    = errors.New("paperless archive representation is absent")
)

// Fetch supports Paperless-ngx v2.13.5, API v5. Version must be the lowercase
// SHA-256 hex digest of the expected representation, not a historical revision.
// Only bounded, validated bytes are returned; HTTP bodies are closed here.
func (f HTTPFetcher) Fetch(req FetchRequest) (FetchResult, error) {
	if len(req.Version) != sha256.Size*2 {
		return FetchResult{}, ErrInvalidConfig
	}
	id, idErr := strconv.ParseUint(req.DocumentID, 10, 64)
	_, digestErr := hex.DecodeString(req.Version)
	if !validText(req.Instance) || idErr != nil || id == 0 || digestErr != nil || req.Version != strings.ToLower(req.Version) || f.Token == "" {
		return FetchResult{}, ErrInvalidConfig
	}
	if req.Representation != OriginalUpload && req.Representation != ArchiveOutput {
		return FetchResult{}, ErrInvalidConfig
	}
	if v := strings.TrimSpace(f.APIVersion); v != "" && v != "5" {
		return FetchResult{}, ErrInvalidConfig
	}
	maxBytes := f.MaxBytes
	if maxBytes == 0 {
		maxBytes = DefaultMaxDocumentBytes
	}
	if maxBytes < 0 || maxBytes == math.MaxInt64 {
		return FetchResult{}, ErrInvalidConfig
	}
	base, err := url.Parse(f.BaseURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" || base.RawQuery != "" || base.Fragment != "" || base.User != nil {
		return FetchResult{}, ErrInvalidConfig
	}
	var archiveChecksum string
	if req.Representation == ArchiveOutput {
		archiveChecksum, err = f.archiveChecksum(base, req.DocumentID)
		if err != nil {
			return FetchResult{}, err
		}
	}
	u := base.JoinPath("api", "documents", req.DocumentID, "download/")
	if req.Representation == OriginalUpload {
		u.RawQuery = "original=true"
	}
	data, err := f.getBytes(u.String(), maxBytes)
	if err != nil {
		return FetchResult{}, err
	}
	digest := sha256.Sum256(data)
	observed := hex.EncodeToString(digest[:])
	if observed != req.Version {
		return FetchResult{}, ErrVersionChanged
	}
	if req.Representation == ArchiveOutput {
		// Paperless's MD5 is only a representation consistency check; SHA-256
		// above binds the captured content to the caller's expected identity.
		checksum := md5.Sum(data)
		if hex.EncodeToString(checksum[:]) != archiveChecksum {
			return FetchResult{}, ErrVersionChanged
		}
		after, err := f.archiveChecksum(base, req.DocumentID)
		if err != nil {
			return FetchResult{}, err
		}
		if after != archiveChecksum {
			return FetchResult{}, ErrVersionChanged
		}
	}
	return FetchResult{Version: observed, Document: bytes.NewReader(data)}, nil
}

func (f HTTPFetcher) archiveChecksum(base *url.URL, id string) (string, error) {
	u := base.JoinPath("api", "documents", id, "metadata/")
	data, err := f.getBytes(u.String(), 1<<20)
	if err != nil {
		return "", err
	}
	var meta struct {
		HasArchiveVersion *bool  `json:"has_archive_version"`
		ArchiveChecksum   string `json:"archive_checksum"`
	}
	if err := json.Unmarshal(data, &meta); err != nil || meta.HasArchiveVersion == nil {
		return "", fmt.Errorf("%w: invalid archive metadata", ErrFetchFailed)
	}
	if !*meta.HasArchiveVersion {
		return "", ErrArchiveMissing
	}
	checksum, err := hex.DecodeString(meta.ArchiveChecksum)
	if err != nil || len(checksum) != md5.Size || meta.ArchiveChecksum != strings.ToLower(meta.ArchiveChecksum) {
		return "", fmt.Errorf("%w: invalid archive checksum", ErrFetchFailed)
	}
	return meta.ArchiveChecksum, nil
}

func (f HTTPFetcher) getBytes(u string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+f.Token)
	req.Header.Set("Accept", "application/json; version=5")
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return nil, fmt.Errorf("%w: status %d", ErrFetchUnauthorized, resp.StatusCode)
		case http.StatusNotFound:
			return nil, fmt.Errorf("%w: status %d", ErrFetchNotFound, resp.StatusCode)
		default:
			return nil, fmt.Errorf("%w: status %d", ErrFetchFailed, resp.StatusCode)
		}
	}
	return boundedBytes(resp.Body, maxBytes)
}
