package ml

// Module encapsulates all required logic regarding the machine learning parameters.
type Module struct {
	Activation
	*Learning
	Descent
}

// Base creates an  ml module with some basic config.
func Base() *Module {
	return &Module{
		Activation: Sigmoid,
		Learning:   Learn(1, 0),
		Descent:    GradientDescent{},
	}
}

// WithRate adjusts the rate for the learning process.
func (ml *Module) WithRate(rate *Learning) *Module {
	ml.Learning = rate
	return ml
}

// WithActivation sets the activation function.
func (ml *Module) WithActivation(activation Activation) *Module {
	ml.Activation = activation
	return ml
}

// WithDescent defines the gradient descent process.
func (ml *Module) WithDescent(descent Descent) *Module {
	ml.Descent = descent
	return ml
}

// NoML creates a void ml module e.g. no learning takes place.
var NoML = Module{
	Activation: Void{},
	Learning:   &Learning{},
	Descent:    Zero{},
}
