package model

// State defines the state of the trader.
// This struct is used to save the configuration in order for the process to keep track of it's state.
type State struct {
	MinSize   int                 `json:"min_size"`
	Running   bool                `json:"running"`
	Positions map[string]Position `json:"positions"`
}
