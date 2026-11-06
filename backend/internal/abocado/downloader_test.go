package abocado

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDownloadFallsBackToHistoryObjectURL(t *testing.T) {
	var requested []string
	d := NewDownloader()
	d.Client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requested = append(requested, r.URL.Host)
		if r.URL.Host == "cdn.abocado.ai" {
			return nil, io.ErrUnexpectedEOF
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("valid-media")), ContentLength: 11, Request: r}, nil
	})
	a := Asset{JobKey: "paid-job", Name: "paid-job.mp4"}
	saved, err := d.DownloadFromSources(context.Background(), a, []string{"https://cdn.abocado.ai/media.mp4", "https://abocado-ai.s3.ap-northeast-2.amazonaws.com/media.mp4"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != "downloaded" || saved.URL != "https://abocado-ai.s3.ap-northeast-2.amazonaws.com/media.mp4" {
		t.Fatalf("unexpected recovered result: %+v", saved)
	}
	if len(requested) != 2 || requested[1] != "abocado-ai.s3.ap-northeast-2.amazonaws.com" {
		t.Fatalf("fallback was not used: %v", requested)
	}
}

func TestDownloaderRejectsUntrustedHostAndRedirect(t *testing.T) {
	d := NewDownloader()
	if _, err := d.Download(context.Background(), Asset{JobKey: "x", URL: "https://cdn.abocado.ai.attacker.test/x", Name: "x.mp4"}, t.TempDir()); err == nil {
		t.Fatal("host suffix spoof was accepted")
	}
	calls := 0
	d.Client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://evil.test/media.mp4"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})
	_, err := d.Download(context.Background(), Asset{JobKey: "x", URL: "https://cdn.abocado.ai/media.mp4", Name: "x.mp4"}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "untrusted asset host") {
		t.Fatalf("untrusted redirect was not rejected: %v", err)
	}
	if calls != 1 {
		t.Fatalf("redirect was followed: %d requests", calls)
	}
}

