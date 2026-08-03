package cliupload

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// PrintAccessLimits fetches group lifecycle limits to stderr for --verbose.
// With token: GET /user/quota. Without: GET /config → guest.
func PrintAccessLimits(ctx context.Context, baseURL, token string) error {
	base, err := NormalizeBaseURL(baseURL)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	tok := strings.TrimSpace(token)
	if tok != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1/user/quota", nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil || !env.Status {
			return fmt.Errorf("quota HTTP %d: %s", resp.StatusCode, truncate(string(raw), 160))
		}
		var q map[string]any
		if err := json.Unmarshal(env.Data, &q); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "verbose: account group limits default_expires_in=%v max_expires_in=%v default_max_views=%v max_max_views=%v force_max_age_days=%v retention_days=%v\n",
			q["default_expires_in"], q["max_expires_in"], q["default_max_views"], q["max_max_views"], q["force_max_age_days"], q["retention_days"])
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1/config", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil || !env.Status {
		return fmt.Errorf("config HTTP %d: %s", resp.StatusCode, truncate(string(raw), 160))
	}
	var cfg map[string]any
	if err := json.Unmarshal(env.Data, &cfg); err != nil {
		return err
	}
	g, _ := cfg["guest"].(map[string]any)
	if g == nil {
		fmt.Fprintln(os.Stderr, "verbose: guest object null (guest upload may be disabled)")
		return nil
	}
	fmt.Fprintf(os.Stderr, "verbose: guest limits max_file_size=%v default_expires_in=%v max_expires_in=%v force_max_age_days=%v max_max_views=%v\n",
		g["max_file_size"], g["default_expires_in"], g["max_expires_in"], g["force_max_age_days"], g["max_max_views"])
	return nil
}
