package chat

import (
	"math/rand"
	"sync"
	"time"
)

// ColorPicker represents a strategy for choosing a display color for new clients.
type ColorPicker interface {
	Next() string
}

var defaultColorPalette = []string{
	"\033[31m", // Red
	"\033[32m", // Green
	"\033[33m", // Yellow
	"\033[34m", // Blue
	"\033[35m", // Magenta
	"\033[36m", // Cyan
}

func newRandomColorPicker(palette []string) ColorPicker {
	if len(palette) == 0 {
		return nil
	}
	return &randomColorPicker{
		palette: append([]string(nil), palette...),
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

type randomColorPicker struct {
	mu      sync.Mutex
	palette []string
	rng     *rand.Rand
}

func (p *randomColorPicker) Next() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.palette) == 0 {
		return ""
	}
	return p.palette[p.rng.Intn(len(p.palette))]
}
