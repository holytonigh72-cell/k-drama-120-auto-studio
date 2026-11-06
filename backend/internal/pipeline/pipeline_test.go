package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failOnce struct{ failed bool }

func (f *failOnce) Run(ctx context.Context, p *Project, s string) ([]string, error) {
	if s == "images" && !f.failed {
		f.failed = true
		return nil, errors.New("simulated transient error")
	}
	return MockRunner{}.Run(ctx, p, s)
}

func TestMockResumeAfterFailure(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	p := NewProject(root, "pilot", "premise")
	runner := &failOnce{}
	if err = p.RunStage(context.Background(), "script", runner, store); err != nil {
		t.Fatal(err)
	}
	if err = p.RunStage(context.Background(), "images", runner, store); err == nil {
		t.Fatal("expected injected failure")
	}
	reloaded, err := store.Get(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Stages["script"].Status != Complete || reloaded.Stages["images"].Status != Failed {
		t.Fatalf("unexpected resume state: %+v", reloaded.Stages)
	}
	if err = reloaded.Resume(context.Background(), MockRunner{}, store); err != nil {
		t.Fatal(err)
	}
	if reloaded.Stages["edit"].Status != Complete {
		t.Fatal("pipeline did not reach edit")
	}
}

func TestBudgetApprovalRequired(t *testing.T) {
	p := NewProject(t.TempDir(), "pilot", "premise")
	p.Provider = "abocado"
	if err := p.CanCharge(); err == nil {
		t.Fatal("unapproved paid request was allowed")
	}
	e := Estimate{Currency: "credits", Total: 100, Balance: 500, Verified: true}
	if err := p.ApproveBudget(e, 80); err == nil {
		t.Fatal("over cap estimate was approved")
	}
	if err := p.ApproveBudget(e, 120); err != nil {
		t.Fatal(err)
	}
	if err := p.CanCharge(); err != nil {
		t.Fatal(err)
	}
}

func TestFFmpegPlanTargetsFiveMinutes(t *testing.T) {
	args, err := FFmpegCommand([]string{"a.mp4", "b.mp4"}, "final.mp4", "1920x1080")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"300", "1920x1080", "libx264"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("plan missing %s: %s", want, joined)
		}
	}
}

func TestFFmpegProjectAddsTimedAudioAndSubtitle(t *testing.T) {
	args, err := FFmpegProjectCommand([]string{"clip01.mp4", "clip02.mp4"}, []AudioClip{{Path: "voice.mp3", StartMS: 12000}}, "captions.srt", "final.mp4", "1920x1080")
	if err != nil {
		t.Fatal(err)
	}
	s := strings.Join(args, " ")
	for _, token := range []string{"adelay=12000|12000", "loudnorm=I=-16", "mov_text", "-t 300", "1920x1080"} {
		if !strings.Contains(s, token) {
			t.Fatalf("render plan missing %q: %s", token, s)
		}
	}
}

func TestLocalAssetDiscovery(t *testing.T) {
	root := t.TempDir()
	assets := filepath.Join(root, "assets")
	if err := os.MkdirAll(assets, 0755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"episode12.mp4", "dialogue.wav", "portrait.png", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(assets, f), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	dest := filepath.Join(root, "imported")
	got, err := ImportAssets([]string{assets}, []string{dest})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 recognized assets, got %d", len(got))
	}
	for _, asset := range got {
		if asset.ImportedPath == "" {
			t.Fatalf("asset was inventoried but not copied: %+v", asset)
		}
	}
	again, err := ImportAssets([]string{assets}, []string{dest})
	if err != nil || len(again) != 3 {
		t.Fatalf("repeat discovery failed: %v", err)
	}
	for i := range got {
		if again[i].ImportedPath != got[i].ImportedPath {
			t.Fatal("duplicate asset copy on repeat scan")
		}
	}
}

func TestSRTValidation(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "captions.srt")
	valid := "1\n00:00:01,000 --> 00:00:02,000\n안녕하세요.\n\n2\n00:00:02,100 --> 00:00:03,000\n반갑습니다.\n"
	if err := os.WriteFile(p, []byte(valid), 0600); err != nil {
		t.Fatal(err)
	}
	cues, err := ParseSRT(p)
	if err != nil || len(cues) != 2 {
		t.Fatalf("valid captions rejected: %v", err)
	}
	invalid := "1\n00:00:01,000 --> 00:00:04,000\n하나\n\n2\n00:00:03,000 --> 00:00:05,000\n둘\n"
	if err := os.WriteFile(p, []byte(invalid), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseSRT(p); err == nil {
		t.Fatal("overlapping captions were accepted")
	}
}

