package abocado

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Asset struct {
	JobKey    string   `json:"job_key"`
	Kind      string   `json:"kind"`
	URL       string   `json:"url"`
	URLs      []string `json:"urls,omitempty"`
	Name      string   `json:"name"`
	Model     string   `json:"model,omitempty"`
	Credits   int      `json:"charged_credits,omitempty"`
	LocalPath string   `json:"local_path,omitempty"`
	Status    string   `json:"status"`
	Error     string   `json:"error,omitempty"`
}
type Downloader struct{ Client *http.Client }

const maxAssetBytes = 512 << 20

func NewDownloader() *Downloader {
	return &Downloader{Client: &http.Client{Timeout: 5 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if !allowedAssetURL(req.URL) {
			return fmt.Errorf("refusing redirect to untrusted asset host")
		}
		if len(via) >= 5 {
			return fmt.Errorf("too many asset redirects")
		}
		return nil
	}}}
}

func allowedAssetURL(u *url.URL) bool {
	if u == nil || u.Scheme != "https" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "cdn.abocado.ai", "abocado-ai.s3.ap-northeast-2.amazonaws.com":
		return true
	default:
		return false
	}
}

func (d *Downloader) Download(ctx context.Context, a Asset, root string) (Asset, error) {
	return d.DownloadFromSources(ctx, a, []string{a.URL}, root)
}

// DownloadFromSources tries the canonical CDN URL and then the original object
// URL from Abocado history. It never follows redirects outside the allowlist.
func (d *Downloader) DownloadFromSources(ctx context.Context, a Asset, sources []string, root string) (Asset, error) {
	if len(sources) == 0 {
		return a, fmt.Errorf("refusing untrusted asset host")
	}
	var failures []string
	seen := map[string]bool{}
	for _, source := range sources {
		if seen[source] {
			continue
		}
		seen[source] = true
		u, err := url.Parse(source)
		if !allowedAssetURL(u) {
			failures = append(failures, "untrusted asset host")
			continue
		}
		a.URL = source
		saved, err := d.downloadOne(ctx, a, root)
		if err == nil {
			return saved, nil
		}
		failures = append(failures, fmt.Sprintf("%s: %v", u.Hostname(), err))
	}
	a.Status = "failed"
	a.Error = strings.Join(failures, "; ")
	return a, fmt.Errorf("all Abocado asset sources failed: %s", a.Error)
}

func (d *Downloader) downloadOne(ctx context.Context, a Asset, root string) (Asset, error) {
	if err := os.MkdirAll(root, 0755); err != nil {
		return a, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return a, err
	}
	resp, err := d.Client.Do(req)
	if err != nil {
		a.Status = "failed"
		a.Error = err.Error()
		return a, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = fmt.Errorf("download returned HTTP %d", resp.StatusCode)
		a.Status = "failed"
		a.Error = err.Error()
		return a, err
	}
	if resp.ContentLength > maxAssetBytes {
		return a, fmt.Errorf("asset exceeds %d-byte safety limit", maxAssetBytes)
	}
	name := filepath.Base(a.Name)
	if name == "." || name == "/" || name == "\\" {
		return a, fmt.Errorf("invalid asset filename")
	}
	sum := sha256.Sum256([]byte(a.JobKey + "|" + a.URL))
	dst := filepath.Join(root, hex.EncodeToString(sum[:6])+"_"+name)
	if st, e := os.Stat(dst); e == nil {
		if st.Size() > 0 {
			a.LocalPath = dst
			a.Status = "downloaded"
			return a, nil
		}
		_ = os.Remove(dst)
	}
	f, e := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if e != nil {
		return a, e
	}
	written, copyErr := io.Copy(f, io.LimitReader(resp.Body, maxAssetBytes+1))
	if copyErr == nil && written > maxAssetBytes {
		copyErr = fmt.Errorf("asset exceeds %d-byte safety limit", maxAssetBytes)
	}
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		a.Status = "failed"
		a.Error = copyErr.Error()
		return a, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dst)
		return a, closeErr
	}
	a.LocalPath = dst
	a.Status = "downloaded"
	return a, nil
}

