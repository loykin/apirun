package basic

import "context"

type Adapter struct{ C Config }

func (m Adapter) Name() string { return m.C.Name }
func (m Adapter) Acquire(_ context.Context) (string, string, error) {
	return AcquireBasic(m.C)
}
