package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"kdrama120/desktopbridge"
)

type App struct {
	ctx     context.Context
	service *desktopbridge.Service
}

func NewApp() *App { return &App{} }
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	root, err := os.UserConfigDir()
	if err != nil {
		root = "."
	}
	a.service = desktopbridge.NewService(filepath.Join(root, "KDrama120"))
}
func (a *App) ListProjects() ([]*desktopbridge.Project, error) { return a.service.ListProjects() }
func (a *App) CreateProject(title, premise string) (*desktopbridge.Project, error) {
	return a.service.CreateProject(title, premise)
}
func (a *App) RunMock(id string) (*desktopbridge.Project, error) { return a.service.RunMock(id) }
func (a *App) ImportLocalAssets(paths []string) ([]desktopbridge.ImportResult, error) {
	return a.service.ImportLocalAssets(paths)
}
func (a *App) ChooseImportFolder() ([]desktopbridge.ImportResult, error) {
	folder, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "기존 드라마 자산 폴더 선택"})
	if err != nil {
		return nil, err
	}
	if folder == "" {
		return nil, nil
	}
	return a.service.ImportLocalAssets([]string{folder})
}
func (a *App) RecoverExistingAssets(configPath string) ([]desktopbridge.RecoveredAsset, error) {
	return a.service.RecoverExistingAssets(configPath)
}
func (a *App) TestAbocadoConnection(configPath string) desktopbridge.ConnectionStatus {
	return a.service.TestAbocadoConnection(configPath)
}
func (a *App) BeginAbocadoAuth(configPath string) (string, error) {
	flow, err := a.service.BeginAbocadoAuth(configPath)
	if err != nil {
		return "", err
	}
	runtime.BrowserOpenURL(a.ctx, flow.URL)
	go func() {
		e := <-flow.Done
		if e != nil {
			runtime.EventsEmit(a.ctx, "abocado:auth-failed", e.Error())
			return
		}
		status := a.service.TestAbocadoConnection(configPath)
		runtime.EventsEmit(a.ctx, "abocado:connected", status)
	}()
	return "인증 페이지를 열었습니다. Windows 브라우저에서 승인한 뒤 이 창으로 돌아오세요.", nil
}
func (a *App) RenderProject(videos []string, audio []desktopbridge.AudioClip, subtitle, output, resolution string) error {
	return a.service.RenderProject(videos, audio, subtitle, output, resolution)
}

