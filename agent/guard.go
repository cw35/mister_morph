package agent

import "github.com/quailyquaily/mistermorph/guard"

func WithGuard(g *guard.Guard) Option {
	return func(e *Engine) {
		e.guard = g
	}
}
