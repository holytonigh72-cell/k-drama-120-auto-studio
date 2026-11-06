// Package desktopbridge is the public Wails boundary for the internal pipeline.
package desktopbridge

import (
	"context"
	"kdrama120/internal/abocado"
	"kdrama120/internal/pipeline"
)

type Service struct{ impl *pipeline.Service }
type Project = pipeline.Project
type ImportResult = pipeline.ImportResult
type RecoveredAsset = pipeline.RecoveredAsset
type AudioClip = pipeline.AudioClip
type ConnectionStatus = pipeline.ConnectionStatus

func NewService(root string) *Service                { return &Service{impl: pipeline.NewService(root)} }
func (s *Service) ListProjects() ([]*Project, error) { return s.impl.ListProjects() }
func (s *Service) CreateProject(title, premise string) (*Project, error) {
	return s.impl.CreateProject(title, premise)
}
func (s *Service) RunMock(id string) (*Project, error) { return s.impl.RunMock(id) }
func (s *Service) ImportLocalAssets(paths []string) ([]ImportResult, error) {
	return s.impl.ImportLocalAssets(paths)
}
func (s *Service) RecoverExistingAssets(configPath string) ([]RecoveredAsset, error) {
	return s.impl.RecoverAbocadoAssets(configPath, "abocado")
}
func (s *Service) TestAbocadoConnection(configPath string) ConnectionStatus {
	return s.impl.TestAbocadoConnection(configPath)
}
func (s *Service) BeginAbocadoAuth(configPath string) (*abocado.OAuthFlow, error) {
	return s.impl.BeginAbocadoAuth(configPath)
}
func (s *Service) SetAbocadoToken(token string) { s.impl.SetAbocadoToken(token) }
func (s *Service) RenderProject(videos []string, audio []AudioClip, subtitle, output, resolution string) error {
	return pipeline.RenderProject(context.Background(), videos, audio, subtitle, output, resolution)
}

