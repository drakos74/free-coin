package stats

import (
	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/buffer"
	cointime "github.com/drakos74/free-coin/internal/time"
)

// Window defines the window stats collector
type Window struct {
	W buffer.HistoryWindow `json:"window"`
	// TODO : see if we can remove the pointer here as well
	C *buffer.HMM `json:"hmm"`
}

// StaticWindow is a static representation of the window state.
// It s used for storing the window state.
type StaticWindow struct {
	W buffer.HistoryWindow `json:"window"`
	C buffer.HMM           `json:"hmm"`
}

func newWindow(cfg processor.Config) Window {
	// find out the max window size
	hmm := make([]buffer.HMMConfig, len(cfg.Stats))
	var windowSize int
	for i, stat := range cfg.Stats {
		ws := stat.LookAhead + stat.LookBack + 1
		if windowSize < ws {
			windowSize = ws
		}
		hmm[i] = buffer.HMMConfig{
			LookBack:  stat.LookBack,
			LookAhead: stat.LookAhead,
		}
	}
	return Window{
		W: buffer.NewHistoryWindow(cointime.ToMinutes(cfg.Duration), windowSize),
		C: buffer.NewMultiHMM(hmm...),
	}
}
