package net

import (
	"github.com/drakos74/go-ex-machina/xmath"
)

type NN interface {
	NetworkConfig
	Train(input xmath.Vector, output xmath.Vector) (err xmath.Vector, weights map[Meta]Weights)
	Predict(input xmath.Vector) xmath.Vector
	GetInfo() Info
}

// Info holds metadata information for the Network
type Info struct {
	Init       bool
	InputSize  int
	OutputSize int
	Iterations int
}

type NetworkConfig interface {
	Debug()
	HasDebugEnabled() bool
	Trace()
	HasTraceEnabled() bool
}

// Config holds generic config data for the network behaviour.
type Config struct {
	trace bool
	debug bool
}

// Debug enables the debug flag for the network.
func (cfg *Config) Debug() {
	cfg.debug = true
}

// Trace enables the trace flag for the network
func (cfg *Config) Trace() {
	cfg.trace = true
}

// HasDebugEnabled defines if the network has the debug flag enabled.
func (cfg *Config) HasDebugEnabled() bool {
	return cfg.debug
}

// HasTraceEnabled defines if the network has th trace flag enabled
func (cfg *Config) HasTraceEnabled() bool {
	return cfg.trace
}
