package pocketbase

import "context"

type Adapter struct{ C Config }

func (m Adapter) Acquire(ctx context.Context) (string, error) {
	return AcquirePocketBase(ctx, m.C)
}
