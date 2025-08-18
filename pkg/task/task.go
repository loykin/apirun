package task

import (
	"context"
	"errors"
)

type Task struct {
	Up   Up   `yaml:"up" json:"up"`
	Down Down `yaml:"down" json:"down"`
}

// UpExecute delegates to the Up spec executor.
func (t Task) UpExecute(ctx context.Context, method, url string) (*ExecResult, error) {
	return t.Up.Execute(ctx, method, url)
}

// ExecuteUp is a convenience wrapper used by tests to run an UpSpec.
func ExecuteUp(ctx context.Context, up UpSpec, method, url string) (*ExecResult, error) {
	return up.Execute(ctx, method, url)
}

// DownExecute is not implemented yet in this project.
// It returns a clear error to indicate the current limitation.
func (t Task) DownExecute(_ context.Context, method, url string) (*ExecResult, error) {
	return nil, errors.New("DownExecute is not implemented")
}
