package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kdrama120/internal/abocado"
)

type Service struct {
	store        *Store
	root         string
	runner       Runner
	abocadoToken string
	vault        abocado.TokenVault
}

func NewService(root string) *Service {
	store, _ := NewStore(filepath.Join(root, "projects"))
	return &Service{store: store, root: root, runner: MockRunner{}, vault: abocado.TokenVault{Path: filepath.Join(root, "abocado.oauth")}}
}

func (s *Service) SetAbocadoToken(token string) { s.abocadoToken = token }

type ConnectionStatus struct {
	State         string `json:"state"`
	Server        string `json:"server,omitempty"`
	Authenticated bool   `json:"authenticated"`
	Message       string `json:"message"`
}

func (s *Service) BeginAbocadoAuth(configPath string) (*abocado.OAuthFlow, error) {
	if configPath == "" {
		configPath = abocado.FirstAvailableConfig()
	}
	if configPath == "" {
		return nil, fmt.Errorf("Codex MCP configuration was not found")
	}
	cfg, err := abocado.LoadServer(configPath, "abocado")
	if err != nil {
		return nil, err
	}
	if cfg.URL == "" {
		return nil, fmt.Errorf("OAuth login requires a remote Codex MCP URL")
	}
	return abocado.BeginOAuth(context.Background(), cfg.URL, s.vault)
}

func (s *Service) TestAbocadoConnection(configPath string) ConnectionStatus {
	if configPath == "" {
		configPath = abocado.FirstAvailableConfig()
	}
	if configPath == "" {
		return ConnectionStatus{State: "not_configured", Message: "Codex MCP configuration was not found"}
	}
	cfg, err := abocado.LoadServer(configPath, "abocado")
	if err != nil {
		return ConnectionStatus{State: "not_configured", Message: err.Error()}
	}
	host := "local stdio MCP"
	if cfg.URL != "" {
		u, e := url.Parse(cfg.URL)
		if e != nil || u.Hostname() == "" {
			return ConnectionStatus{State: "invalid_config", Message: "Abocado MCP URL is invalid"}
		}
		host = u.Hostname()
		dnsCtx, dnsCancel := context.WithTimeout(context.Background(), 4*time.Second)
		_, e = net.DefaultResolver.LookupHost(dnsCtx, host)
		if e != nil {
			controlCtx, controlCancel := context.WithTimeout(context.Background(), 3*time.Second)
			_, controlErr := net.DefaultResolver.LookupHost(controlCtx, "example.com")
			controlCancel()
			dnsCancel()
			if controlErr != nil {
				return ConnectionStatus{State: "dns_unavailable", Server: host, Message: "Windows DNS could not resolve Abocado or a public connectivity check. Check this PC's network/DNS, then run the in-app test again."}
			}
			return ConnectionStatus{State: "host_unresolved", Server: host, Message: "Public DNS works, but the Abocado MCP host did not resolve."}
		}
		dnsCancel()
	}
	tokens, e := s.vault.LoadFresh(context.Background())
	if e == nil {
		s.abocadoToken = tokens.AccessToken
	}
	client, e := abocado.StartAuthorized(cfg, "abocado", s.abocadoToken)
	if e != nil {
		var status *abocado.HTTPStatusError
		if errors.As(e, &status) && (status.StatusCode == http.StatusUnauthorized || status.StatusCode == http.StatusForbidden) {
			return ConnectionStatus{State: "authentication_required", Server: host, Message: "Abocado requires one-time authorization. Use Connect in this window."}
		}
		return ConnectionStatus{State: "unavailable", Server: host, Authenticated: s.abocadoToken != "", Message: e.Error()}
	}
	defer client.Close()
	tools, e := client.ListTools()
	if e != nil {
		return ConnectionStatus{State: "unavailable", Server: host, Authenticated: s.abocadoToken != "", Message: e.Error()}
	}
	if e = abocado.ValidateRecoveryTools(tools); e != nil {
		return ConnectionStatus{State: "connected_limited", Server: host, Authenticated: s.abocadoToken != "", Message: e.Error()}
	}
	return ConnectionStatus{State: "connected", Server: host, Authenticated: true, Message: "Abocado MCP is reachable; history and status tools are ready. No generation was submitted."}
}
func (s *Service) ListProjects() ([]*Project, error) { return s.store.List() }
func (s *Service) CreateProject(title, premise string) (*Project, error) {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(premise) == "" {
		return nil, fmt.Errorf("제목과 주제를 입력하세요")
	}
	p := NewProject(s.root, title, premise)
	if err := s.store.Save(p); err != nil {
		return nil, err
	}
	return p, nil
}
func (s *Service) RunMock(id string) (*Project, error) {
	p, err := s.store.Get(id)
	if err != nil {
		return nil, err
	}
	if err = p.Resume(context.Background(), s.runner, s.store); err != nil {
		return p, err
	}
	return p, nil
}
func (s *Service) ImportLocalAssets(paths []string) ([]ImportResult, error) {
	return ImportAssets(paths, []string{filepath.Join(s.root, "imported")})
}
func (s *Service) ApproveBudget(id string, e Estimate, cap float64) (*Project, error) {
	p, err := s.store.Get(id)
	if err != nil {
		return nil, err
	}
	if err = p.ApproveBudget(e, cap); err != nil {
		return p, err
	}
	if err = s.store.Save(p); err != nil {
		return p, err
	}
	return p, nil
}

type RecoveredAsset struct {
	JobKey    string   `json:"job_key"`
	Kind      string   `json:"kind"`
	Model     string   `json:"model"`
	Credits   int      `json:"charged_credits"`
	URL       string   `json:"url"`
	Sources   []string `json:"sources,omitempty"`
	LocalPath string   `json:"local_path,omitempty"`
	Status    string   `json:"status"`
	Error     string   `json:"error,omitempty"`
}
type mcpToolEnvelope struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Structured json.RawMessage `json:"structuredContent"`
	IsError    bool            `json:"isError"`
}

func toolData(raw json.RawMessage) (json.RawMessage, error) {
	var e mcpToolEnvelope
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil, err
	}
	if e.IsError {
		return nil, fmt.Errorf("Abocado MCP tool returned an error")
	}
	if len(e.Structured) > 0 {
		return e.Structured, nil
	}
	for _, c := range e.Content {
		if c.Type == "text" {
			var v any
			if json.Unmarshal([]byte(c.Text), &v) == nil {
				return json.Marshal(v)
			}
		}
	}
	return nil, fmt.Errorf("MCP response did not include structured data")
}

// RecoverAbocadoAssets reads only completed history; it never submits generation jobs.
// It selects the already completed 12-second Seedance clip and four most recent
// Korean Gemini TTS jobs, then downloads their existing result URLs.
func (s *Service) RecoverAbocadoAssets(configPath, serverName string) ([]RecoveredAsset, error) {
	if configPath == "" {
		configPath = abocado.FirstAvailableConfig()
	}
	if configPath == "" {
		return nil, fmt.Errorf("Codex MCP config was not found")
	}
	if serverName == "" {
		serverName = "abocado"
	}
	cfg, err := abocado.LoadServer(configPath, serverName)
	if err != nil {
		return nil, err
	}
	if tokens, tokenErr := s.vault.LoadFresh(context.Background()); tokenErr == nil {
		s.abocadoToken = tokens.AccessToken
	}
	client, err := abocado.StartAuthorized(cfg, serverName, s.abocadoToken)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	raw, err := client.ListTools()
	if err != nil {
		return nil, err
	}
	if err = abocado.ValidateRecoveryTools(raw); err != nil {
		return nil, err
	}
	type candidate struct {
		JobKey, Model, Kind string
		Credits             int
	}
	// These are the specific already-paid results for this pilot. Recovery is
	// intentionally keyed by immutable job IDs and never searches for a substitute.
	selected := []candidate{
		{"mcp_google_8ef4086c_20261106020452_8uaaro", "se-bytedance-seedance-v2-mini-reference-i2v", "video", 2410},
		{"mcp_google_8ef4086c_20261106154013_oaepbc", "se-gemini-3.1-flash-t2a", "audio", 41},
		{"mcp_google_8ef4086c_20261106154015_mr519y", "se-gemini-3.1-flash-t2a", "audio", 38},
		{"mcp_google_8ef4086c_20261106154017_yi9gzw", "se-gemini-3.1-flash-t2a", "audio", 101},
		{"mcp_google_8ef4086c_20261106154019_mfu58e", "se-gemini-3.1-flash-t2a", "audio", 14},
	}
	historySources, historyErr := s.abocadoHistorySources(client)
	if historyErr != nil && len(historySources) == 0 {
		// Job status still provides the CDN URL. History is used only to add the
		// S3-origin fallback, so it must not prevent recovery from being attempted.
		historySources = map[string][]string{}
	}
	dest := filepath.Join(s.root, "assets", "recovered")
	downloader := abocado.NewDownloader()
	var out []RecoveredAsset
	for _, j := range selected {
		statusRaw, e := client.CallTool("abocado_get_job_status", map[string]any{"job_key": j.JobKey})
		if e != nil {
			out = append(out, RecoveredAsset{JobKey: j.JobKey, Kind: j.Kind, Model: j.Model, Credits: j.Credits, Status: "status_error", Error: e.Error()})
			continue
		}
		statusData, e := toolData(statusRaw)
		if e != nil {
			out = append(out, RecoveredAsset{JobKey: j.JobKey, Kind: j.Kind, Model: j.Model, Credits: j.Credits, Status: "status_error", Error: e.Error()})
			continue
		}
		var status struct {
			Phase  string `json:"phase"`
			Result struct {
				URLs []string `json:"urls"`
			} `json:"result"`
		}
		if e = json.Unmarshal(statusData, &status); e != nil || status.Phase != "done" || len(status.Result.URLs) == 0 {
			out = append(out, RecoveredAsset{JobKey: j.JobKey, Kind: j.Kind, Model: j.Model, Credits: j.Credits, Status: "not_ready"})
			continue
		}
		sources := append([]string{}, status.Result.URLs...)
		sources = append(sources, historySources[j.JobKey]...)
		ext := ".mp4"
		if j.Kind == "audio" {
			ext = ".mp3"
		}
		a := abocado.Asset{JobKey: j.JobKey, Kind: j.Kind, URL: sources[0], URLs: sources, Name: j.JobKey + ext, Model: j.Model, Credits: j.Credits}
		saved, e := downloader.DownloadFromSources(context.Background(), a, sources, dest)
		ra := RecoveredAsset{JobKey: j.JobKey, Kind: j.Kind, Model: j.Model, Credits: j.Credits, URL: a.URL, Sources: sources, LocalPath: saved.LocalPath, Status: saved.Status}
		if e != nil {
			ra.Error = e.Error()
		}
		out = append(out, ra)
	}
	if err = os.MkdirAll(dest, 0755); err != nil {
		return out, err
	}
	manifest, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return out, err
	}
	if err = os.WriteFile(filepath.Join(dest, "recovered_assets.json"), manifest, 0600); err != nil {
		return out, err
	}
	return out, nil
}

func (s *Service) abocadoHistorySources(client *abocado.Client) (map[string][]string, error) {
	var result struct {
		List []struct {
			Prompt struct {
				ExternalPromptID string `json:"external_prompt_id"`
			} `json:"prompt_metadata"`
			Images []struct {
				CDNURL    string `json:"cdn_url"`
				ObjectURL string `json:"object_url"`
			} `json:"images"`
		} `json:"list"`
	}
	known := map[string]bool{
		"mcp_google_8ef4086c_20261106020452_8uaaro": true,
		"mcp_google_8ef4086c_20261106154013_oaepbc": true,
		"mcp_google_8ef4086c_20261106154015_mr519y": true,
		"mcp_google_8ef4086c_20261106154017_yi9gzw": true,
		"mcp_google_8ef4086c_20261106154019_mfu58e": true,
	}
	out := map[string][]string{}
	for page := 1; page <= 20; page++ {
		raw, err := client.CallTool("abocado_get_history", map[string]any{"limit": 50, "page": page})
		if err != nil {
			return out, err
		}
		data, err := toolData(raw)
		if err != nil {
			return out, err
		}
		if err = json.Unmarshal(data, &result); err != nil {
			return out, err
		}
		if len(result.List) == 0 {
			break
		}
		for _, job := range result.List {
			id := job.Prompt.ExternalPromptID
			if !known[id] {
				continue
			}
			for _, media := range job.Images {
				for _, candidate := range []string{media.CDNURL, media.ObjectURL} {
					if candidate != "" && !containsString(out[id], candidate) {
						out[id] = append(out[id], candidate)
					}
				}
			}
		}
		if len(out) == len(known) {
			break
		}
	}
	return out, nil
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

