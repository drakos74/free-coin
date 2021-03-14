package stats

import (
	"fmt"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog/log"
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

func newWindow(key model.Key, cfg processor.Config, store storage.Persistence) Window {
	// find out the max window size
	hmm := make([]buffer.HMMConfig, len(cfg.Model.Stats))
	var windowSize int
	for i, stat := range cfg.Model.Stats {
		ws := stat.LookAhead + stat.LookBack + 1
		if windowSize < ws {
			windowSize = ws
		}
		hmm[i] = buffer.HMMConfig{
			LookBack:  stat.LookBack,
			LookAhead: stat.LookAhead,
		}
	}
	// check if we have a reference to a stored instance
	if cfg.Model.Index > 0 {
		key.Index = cfg.Model.Index
		var window StaticWindow
		err := store.Load(processor.NewStateKey(ProcessorName, key), &window)
		log.Info().
			Err(err).
			Str("key", fmt.Sprintf("%+v", key)).
			Int64("index", cfg.Model.Index).
			Msg("loading previous state for HMM")
		if err != nil {
			return Window{
				W: buffer.NewHistoryWindow(cointime.ToMinutes(cfg.Duration), windowSize),
				C: buffer.HMMFromState(window.C),
			}
		}
	}
	log.Info().
		Str("key", fmt.Sprintf("%+v", key)).
		Int64("index", cfg.Model.Index).
		Msg("start new HMM")
	return Window{
		W: buffer.NewHistoryWindow(cointime.ToMinutes(cfg.Duration), windowSize),
		C: buffer.NewMultiHMM(hmm...),
	}
}
