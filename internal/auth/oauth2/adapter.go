package oauth2

import "context"

type Adapter struct {
	M Method
}

func (a Adapter) Acquire(ctx context.Context) (string, error) {
	return a.M.Acquire(ctx)
}
