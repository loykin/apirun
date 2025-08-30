package basic

import "context"

type Adapter struct{ C Config }

func (m Adapter) Acquire(_ context.Context) (string, error) {
	return AcquireBasic(m.C)
}
