package abocado

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Asset struct {
	JobKey    string `json:"job_key"`
	Kind      string `json:"kind"`
	URL       string `json:"url"`
	Name      string `json:"name"`
	Model     string `json:"model,omitempty"`
	Credits   int    `json:"charged_credits,omitempty"`
	LocalPath string `json:"local_path,omitempty"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}
type Downloader struct{ Client *http.Client }

func NewDownloader() *Downloader { return &Downloader{Client: &http.Client{Timeout: 5 * time.Minute}} }
func (d *Downloader) Download(ctx context.Context, a Asset, root string) (Asset, error) {
	if !strings.HasPrefix(a.URL, "https://cdn.abocado.ai/") && !strings.HasPrefix(a.URL, "https://abocado-ai.s3.ap-northeast-2.amazonaws.com/") {
		return a, fmt.Errorf("refusing untrusted asset host")
	}
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
	name := filepath.Base(a.Name)
	if name == "." || name == "/" || name == "\\" {
		return a, fmt.Errorf("invalid asset filename")
	}
	sum := sha256.Sum256([]byte(a.JobKey + "|" + a.URL))
	dst := filepath.Join(root, hex.EncodeToString(sum[:6])+"_"+name)
	if _, e := os.Stat(dst); e == nil {
		a.LocalPath = dst
		a.Status = "downloaded"
		return a, nil
	}
	tmp := dst + ".partial"
	f, e := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if e != nil {
		return a, e
	}
	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		a.Status = "failed"
		a.Error = copyErr.Error()
		return a, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return a, closeErr
	}
	if e = os.Rename(tmp, dst); e != nil {
		return a, e
	}
	a.LocalPath = dst
	a.Status = "downloaded"
	return a, nil
}

