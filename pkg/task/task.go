package task

import (
	"context"
)

type Task struct {
	Up   Up   `yaml:"up" json:"up"`
	Down Down `yaml:"down" json:"down"`
}

// UpExecute delegates to the Up spec executor.
func (t Task) UpExecute(ctx context.Context, method, url string) (*ExecResult, error) {
	return t.Up.Execute(ctx, method, url)
}

// DownExecute delegates to the Down spec executor.
// The method and url parameters are currently unused because Down includes
// its own Method and URL fields; they are kept for symmetry with UpExecute
// and potential future overrides.
func (t Task) DownExecute(ctx context.Context, _ string, _ string) (*ExecResult, error) {
	return t.Down.Execute(ctx)
}
