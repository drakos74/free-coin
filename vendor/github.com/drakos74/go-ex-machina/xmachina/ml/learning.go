package ml

// Learning defines the learning rates for weight matrices and bias vectors.
type Learning struct {
	wRate float64
	bRate float64
}

// Learn creates a new learning struct.
func Learn(wRate, bRate float64) *Learning {
	return &Learning{wRate: wRate, bRate: bRate}
}

func Rate(rate float64) *Learning {
	return &Learning{wRate: rate, bRate: rate}
}

// WRate returns the weights learning rate.
func (c *Learning) WRate() float64 {
	return c.wRate
}

// BRate returns the bias learning rate.
func (c *Learning) BRate() float64 {
	return c.bRate
}
