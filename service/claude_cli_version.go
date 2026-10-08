package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	claudeCLILatestVersionURL = "https://registry.npmjs.org/@anthropic-ai/claude-code/latest"
	claudeCLIVersionCacheTTL  = time.Hour
)

type claudeCLIVersionCache struct {
	sync.Mutex
	version   string
	expiresAt time.Time
}

var latestClaudeCLIVersion claudeCLIVersionCache

func GetLatestClaudeCLIVersion(ctx context.Context, client *http.Client) (string, error) {
	return latestClaudeCLIVersion.get(ctx, client, claudeCLILatestVersionURL, time.Now())
}

func (cache *claudeCLIVersionCache) get(ctx context.Context, client *http.Client, registryURL string, now time.Time) (string, error) {
	cache.Lock()
	defer cache.Unlock()

	if now.Before(cache.expiresAt) {
		if cache.version == "" {
			return "", fmt.Errorf("claude cli version lookup failed recently, retry after %s", cache.expiresAt.Format(time.RFC3339))
		}
		return cache.version, nil
	}

	version, err := fetchLatestClaudeCLIVersion(ctx, client, registryURL)
	if err != nil {
		cache.expiresAt = now.Add(claudeCLIVersionCacheTTL)
		if cache.version != "" {
			return cache.version, nil
		}
		return "", err
	}

	cache.version = version
	cache.expiresAt = now.Add(claudeCLIVersionCacheTTL)
	return version, nil
}

func fetchLatestClaudeCLIVersion(ctx context.Context, client *http.Client, registryURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, registryURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("claude cli version lookup failed: status=%d", resp.StatusCode)
	}

	var pkg struct {
		Version string `json:"version"`
	}
	if err := common.DecodeJson(resp.Body, &pkg); err != nil {
		return "", err
	}
	version := strings.TrimSpace(pkg.Version)
	if version == "" {
		return "", fmt.Errorf("claude cli latest package has no version")
	}
	return version, nil
}

func claudeCLIUserAgent(version string) string {
	return "claude-cli/" + version + " (external, cli)"
}
