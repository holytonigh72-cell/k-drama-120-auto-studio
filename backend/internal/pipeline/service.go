package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kdrama120/internal/abocado"
)

type Service struct {
	store  *Store
	root   string
	runner Runner
}

func NewService(root string) *Service {
	store, _ := NewStore(filepath.Join(root, "projects"))
	return &Service{store: store, root: root, runner: MockRunner{}}
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
	JobKey    string `json:"job_key"`
	Kind      string `json:"kind"`
	Model     string `json:"model"`
	Credits   int    `json:"charged_credits"`
	URL       string `json:"url"`
	LocalPath string `json:"local_path,omitempty"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
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
	client, err := abocado.Start(cfg, serverName)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	raw, err := client.ListTools()
	if err != nil {
		return nil, err
	}
	if err = abocado.ValidateTools(raw); err != nil {
		return nil, err
	}
	jobsRaw, err := client.CallTool("abocado_list_jobs", map[string]any{"limit": 100, "phase_filter": "done"})
	if err != nil {
		return nil, err
	}
	data, err := toolData(jobsRaw)
	if err != nil {
		return nil, err
	}
	var listed struct {
		Jobs []struct {
			JobKey  string `json:"job_key"`
			Model   string `json:"model"`
			Type    string `json:"type"`
			Phase   string `json:"phase"`
			Credits int    `json:"charged_credits"`
			Created string `json:"created_at"`
		} `json:"jobs"`
	}
	if err = json.Unmarshal(data, &listed); err != nil {
		return nil, err
	}
	type candidate struct {
		JobKey, Model, Kind string
		Credits             int
		Created             string
	}
	var videos, audios []candidate
	for _, j := range listed.Jobs {
		if j.Phase != "done" {
			continue
		}
		if j.Type == "video" && strings.Contains(j.Model, "seedance-v2-mini-reference-i2v") {
			videos = append(videos, candidate{j.JobKey, j.Model, "video", j.Credits, j.Created})
		}
		if j.Type == "audio" && j.Model == "se-gemini-3.1-flash-t2a" {
			audios = append(audios, candidate{j.JobKey, j.Model, "audio", j.Credits, j.Created})
		}
	}
	sort.Slice(videos, func(i, j int) bool { return videos[i].Created > videos[j].Created })
	sort.Slice(audios, func(i, j int) bool { return audios[i].Created > audios[j].Created })
	if len(videos) > 1 {
		videos = videos[:1]
	}
	if len(audios) > 4 {
		audios = audios[:4]
	}
	selected := append(videos, audios...)
	if len(selected) == 0 {
		return nil, fmt.Errorf("no matching completed video/TTS jobs found")
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
		ext := ".mp4"
		if j.Kind == "audio" {
			ext = ".mp3"
		}
		a := abocado.Asset{JobKey: j.JobKey, Kind: j.Kind, URL: status.Result.URLs[0], Name: j.JobKey + ext, Model: j.Model, Credits: j.Credits}
		saved, e := downloader.Download(context.Background(), a, dest)
		ra := RecoveredAsset{JobKey: j.JobKey, Kind: j.Kind, Model: j.Model, Credits: j.Credits, URL: a.URL, LocalPath: saved.LocalPath, Status: saved.Status}
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

