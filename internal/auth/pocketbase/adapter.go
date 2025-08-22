package pocketbase

import "C"
import "context"

type Adapter struct{ C Config }

func (m Adapter) Name() string { return m.C.Name }
func (m Adapter) Acquire(ctx context.Context) (string, string, error) {
	return AcquirePocketBase(ctx, m.C)
}
