package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const TargetSeconds = 300
const ClipSeconds = 12

var Stages = []string{"script", "images", "video", "tts", "lipsync", "subtitles", "edit"}

type Status string

const (
	Pending  Status = "pending"
	Running  Status = "running"
	Complete Status = "complete"
	Failed   Status = "failed"
	Blocked  Status = "blocked"
)

type Stage struct {
	Name      string    `json:"name"`
	Status    Status    `json:"status"`
	Attempts  int       `json:"attempts"`
	Error     string    `json:"error,omitempty"`
	Output    []string  `json:"output,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Estimate struct {
	Currency string             `json:"currency"`
	Total    float64            `json:"total"`
	Balance  float64            `json:"balance"`
	Items    map[string]float64 `json:"items"`
	Verified bool               `json:"verified"`
}

type Project struct {
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	Premise    string            `json:"premise"`
	Workspace  string            `json:"workspace"`
	Output     string            `json:"output"`
	TargetSec  int               `json:"target_seconds"`
	Resolution string            `json:"resolution"`
	Provider   string            `json:"provider"`
	Cap        float64           `json:"approved_cap"`
	Estimate   *Estimate         `json:"estimate,omitempty"`
	Stages     map[string]*Stage `json:"stages"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

type Store struct {
	mu   sync.Mutex
	root string
}

func NewStore(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}

func (s *Store) path(id string) string { return filepath.Join(s.root, id+".json") }

func (s *Store) Save(p *Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.UpdatedAt = time.Now().UTC()
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(p.ID), b, 0600)
}

func (s *Store) Get(id string) (*Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.path(id))
	if err != nil {
		return nil, err
	}
	var p Project
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) List() ([]*Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	var out []*Project
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, er := os.ReadFile(filepath.Join(s.root, e.Name()))
		if er != nil {
			continue
		}
		var p Project
		er = json.Unmarshal(b, &p)
		if er == nil {
			out = append(out, &p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func NewProject(root, title, premise string) *Project {
	now := time.Now().UTC()
	id := fmt.Sprintf("kd-%d", now.UnixNano())
	stages := map[string]*Stage{}
	for _, n := range Stages {
		stages[n] = &Stage{Name: n, Status: Pending, UpdatedAt: now}
	}
	return &Project{ID: id, Title: title, Premise: premise, Workspace: root, Output: filepath.Join(root, "output", id+".mp4"), TargetSec: TargetSeconds, Resolution: "1920x1080", Provider: "mock", Stages: stages, CreatedAt: now, UpdatedAt: now}
}

func (p *Project) ApproveBudget(e Estimate, cap float64) error {
	if !e.Verified || e.Currency == "" {
		return errors.New("검증된 전체 견적이 필요합니다")
	}
	if e.Total < 0 || cap < 0 {
		return errors.New("예산은 음수일 수 없습니다")
	}
	if e.Total > cap {
		return fmt.Errorf("견적 %.2f %s이 승인 한도 %.2f를 초과합니다", e.Total, e.Currency, cap)
	}
	if e.Total > e.Balance {
		return errors.New("현재 잔액보다 큰 견적은 승인할 수 없습니다")
	}
	p.Estimate = &e
	p.Cap = cap
	return nil
}

func (p *Project) CanCharge() error {
	if p.Provider == "mock" {
		return nil
	}
	if p.Estimate == nil || !p.Estimate.Verified {
		return errors.New("유료 작업 전 검증된 전체 견적과 승인이 필요합니다")
	}
	if p.Estimate.Total > p.Cap {
		return errors.New("견적이 승인 한도를 초과합니다")
	}
	if p.Estimate.Total > p.Estimate.Balance {
		return errors.New("잔액이 부족합니다")
	}
	return nil
}

type Runner interface {
	Run(context.Context, *Project, string) ([]string, error)
}

type MockRunner struct{}

func (MockRunner) Run(_ context.Context, p *Project, stage string) ([]string, error) {
	switch stage {
	case "script":
		return []string{"story_bible.md", "screenplay.md", "character_bible.md", "shot_manifest.csv"}, nil
	case "images":
		return []string{"mock://images/25-placeholder-slates"}, nil
	case "video":
		return []string{"mock://video/25x12s-slates"}, nil
	case "tts":
		return []string{"mock://audio/korean-dialogue.wav"}, nil
	case "lipsync":
		return []string{"mock://video/lipsync-review-required"}, nil
	case "subtitles":
		return []string{"captions.srt"}, nil
	case "edit":
		return []string{"ffmpeg-plan.txt", "target-runtime:300s", "resolution:" + p.Resolution}, nil
	default:
		return nil, fmt.Errorf("알 수 없는 단계: %s", stage)
	}
}

func (p *Project) RunStage(ctx context.Context, name string, runner Runner, store *Store) error {
	st, ok := p.Stages[name]
	if !ok {
		return fmt.Errorf("알 수 없는 단계: %s", name)
	}
	if st.Status == Complete {
		return nil
	}
	if p.Provider != "mock" {
		if err := p.CanCharge(); err != nil {
			st.Status = Blocked
			st.Error = err.Error()
			_ = store.Save(p)
			return err
		}
	}
	for _, prior := range Stages {
		if prior == name {
			break
		}
		if p.Stages[prior].Status != Complete {
			return fmt.Errorf("선행 단계 %s가 완료되지 않았습니다", prior)
		}
	}
	st.Status = Running
	st.Error = ""
	st.Attempts++
	st.UpdatedAt = time.Now().UTC()
	_ = store.Save(p)
	out, err := runner.Run(ctx, p, name)
	st.UpdatedAt = time.Now().UTC()
	if err != nil {
		st.Status = Failed
		st.Error = err.Error()
		_ = store.Save(p)
		return err
	}
	st.Status = Complete
	st.Output = out
	return store.Save(p)
}

func (p *Project) Resume(ctx context.Context, runner Runner, store *Store) error {
	for _, s := range Stages {
		if p.Stages[s].Status != Complete {
			if err := p.RunStage(ctx, s, runner, store); err != nil {
				return err
			}
		}
	}
	return nil
}

type ImportResult struct {
	Path         string `json:"path"`
	ImportedPath string `json:"imported_path,omitempty"`
	Kind         string `json:"kind"`
	Reused       bool   `json:"reused"`
}

var assetExt = map[string]string{".png": "image", ".jpg": "image", ".jpeg": "image", ".webp": "image", ".mp4": "video", ".mov": "video", ".wav": "audio", ".mp3": "audio", ".m4a": "audio", ".srt": "subtitle", ".md": "script", ".csv": "manifest"}

func ImportAssets(searchRoots, destRoots []string) ([]ImportResult, error) {
	var results []ImportResult
	seen := map[string]bool{}
	for _, dest := range destRoots {
		if dest != "" {
			if err := os.MkdirAll(dest, 0755); err != nil {
				return nil, err
			}
		}
	}
	for _, root := range searchRoots {
		if root == "" {
			continue
		}
		abs, _ := filepath.Abs(root)
		for _, excluded := range destRoots {
			d, _ := filepath.Abs(excluded)
			if abs == d || strings.HasPrefix(abs, d+string(os.PathSeparator)) {
				continue
			}
		}
		_ = filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "$Recycle.Bin" {
					return filepath.SkipDir
				}
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			kind, ok := assetExt[ext]
			if !ok {
				return nil
			}
			base := strings.ToLower(filepath.Base(path))
			if strings.Contains(base, "contract marriage") || strings.Contains(base, "계약결혼") || strings.Contains(base, "secret husband") {
				kind = kind + ":pilot"
			}
			rel := strings.TrimPrefix(path, abs)
			rel = strings.TrimLeft(rel, "\\/")
			key := strings.ToLower(rel)
			if seen[key] {
				return nil
			}
			seen[key] = true
			r := ImportResult{Path: path, Kind: kind, Reused: true}
			hash := sha256.New()
			file, openErr := os.Open(path)
			if openErr == nil {
				_, _ = io.Copy(hash, file)
				_ = file.Close()
			}
			digest := hex.EncodeToString(hash.Sum(nil))
			for _, dest := range destRoots {
				if dest == "" {
					continue
				}
				target := filepath.Join(dest, digest[:16]+"_"+filepath.Base(path))
				if _, statErr := os.Stat(target); statErr == nil {
					r.ImportedPath = target
					break
				}
				if copyErr := copyFile(path, target); copyErr == nil {
					r.ImportedPath = target
					break
				}
			}
			results = append(results, r)
			return nil
		})
	}
	return results, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		return copyErr
	}
	return closeErr
}

func FFmpegCommand(inputs []string, output string, resolution string) ([]string, error) {
	if len(inputs) == 0 {
		return nil, errors.New("편집할 영상이 없습니다")
	}
	if resolution != "1920x1080" && resolution != "1280x720" {
		return nil, errors.New("지원 해상도는 1920x1080 또는 1280x720입니다")
	}
	args := []string{"-y"}
	for _, in := range inputs {
		args = append(args, "-i", in)
	}
	args = append(args, "-filter_complex", fmt.Sprintf("concat=n=%d:v=1:a=0,scale=%s:force_original_aspect_ratio=decrease,pad=%s:(ow-iw)/2:(oh-ih)/2,trim=duration=300,setpts=PTS-STARTPTS", len(inputs), resolution, resolution), "-t", "300", "-r", "24", "-an", "-c:v", "libx264", "-pix_fmt", "yuv420p", output)
	return args, nil
}

func Render(ctx context.Context, inputs []string, output, resolution string) error {
	args, err := FFmpegCommand(inputs, output, resolution)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg 실패: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

type AudioClip struct {
	Path    string  `json:"path"`
	StartMS int     `json:"start_ms"`
	GainDB  float64 `json:"gain_db,omitempty"`
}

// FFmpegProjectCommand builds the final 300-second H.264/AAC MP4 and muxes
// timed voice clips and optional SRT captions. Each voice clip's start time is
// explicit so dialogue can be aligned after review rather than guessed.
func FFmpegProjectCommand(videos []string, audio []AudioClip, subtitle, output, resolution string) ([]string, error) {
	if len(videos) == 0 {
		return nil, errors.New("편집할 영상이 없습니다")
	}
	if resolution != "1920x1080" && resolution != "1280x720" {
		return nil, errors.New("지원 해상도는 1920x1080 또는 1280x720입니다")
	}
	for _, a := range audio {
		if a.Path == "" || a.StartMS < 0 || a.StartMS >= TargetSeconds*1000 {
			return nil, errors.New("오디오 파일 경로 또는 시작 타임코드가 잘못됐습니다")
		}
	}
	args := []string{"-y"}
	for _, p := range videos {
		args = append(args, "-i", p)
	}
	for _, a := range audio {
		args = append(args, "-i", a.Path)
	}
	hasSubtitle := subtitle != ""
	if hasSubtitle {
		args = append(args, "-i", subtitle)
	}
	filters := make([]string, 0, len(videos)+len(audio)+2)
	labels := make([]string, 0, len(videos))
	for i := range videos {
		label := fmt.Sprintf("v%d", i)
		filters = append(filters, fmt.Sprintf("[%d:v]fps=24,scale=%s:force_original_aspect_ratio=decrease,pad=%s:(ow-iw)/2:(oh-ih)/2,setsar=1,format=yuv420p,setpts=PTS-STARTPTS[%s]", i, resolution, resolution, label))
		labels = append(labels, "["+label+"]")
	}
	filters = append(filters, strings.Join(labels, "")+fmt.Sprintf("concat=n=%d:v=1:a=0,trim=duration=300,setpts=PTS-STARTPTS[vout]", len(videos)))
	if len(audio) == 0 {
		filters = append(filters, "anullsrc=r=48000:cl=stereo,atrim=duration=300[aout]")
	} else {
		alabels := make([]string, 0, len(audio))
		base := len(videos)
		for i, a := range audio {
			label := fmt.Sprintf("a%d", i)
			delay := a.StartMS
			filters = append(filters, fmt.Sprintf("[%d:a]aresample=48000,asetpts=PTS-STARTPTS,volume=%gdB,adelay=%d|%d[%s]", base+i, a.GainDB, delay, delay, label))
			alabels = append(alabels, "["+label+"]")
		}
		filters = append(filters, strings.Join(alabels, "")+fmt.Sprintf("amix=inputs=%d:duration=longest:normalize=0,apad,atrim=duration=300,loudnorm=I=-16:LRA=11:TP=-1.5[aout]", len(audio)))
	}
	args = append(args, "-filter_complex", strings.Join(filters, ";"), "-map", "[vout]", "-map", "[aout]")
	if hasSubtitle {
		subIndex := len(videos) + len(audio)
		args = append(args, "-map", fmt.Sprintf("%d:0", subIndex), "-c:s", "mov_text")
	}
	args = append(args, "-t", "300", "-r", "24", "-c:v", "libx264", "-preset", "medium", "-crf", "20", "-pix_fmt", "yuv420p", "-c:a", "aac", "-b:a", "192k", "-movflags", "+faststart", output)
	return args, nil
}

func RenderProject(ctx context.Context, videos []string, audio []AudioClip, subtitle, output, resolution string) error {
	args, err := FFmpegProjectCommand(videos, audio, subtitle, output, resolution)
	if err != nil {
		return err
	}
	if subtitle != "" {
		if _, err = ParseSRT(subtitle); err != nil {
			return fmt.Errorf("자막 검증 실패: %w", err)
		}
	}
	if err = os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg 실패: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

