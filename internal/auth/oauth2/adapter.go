package oauth2

import "context"

type Adapter struct{ M Method }

func (a Adapter) Name() string { return a.M.Name() }
func (a Adapter) Acquire(ctx context.Context) (string, string, error) {
	return a.M.Acquire(ctx)
}
