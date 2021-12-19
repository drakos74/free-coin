# Neural Network

A neural network is made up from layers. Each layer has a number or combination of neurons. The neurons can be rather
complex computation units comprised of different cells.

## Neuron

A neuron is the smallest part of computation within a network. For uniformity, it must adhere to a common interface.

```go
// Neuron is a minimal computation unit with an activation function.
// It is effectively a collection of perceptrons so not the smallest unit after all,
// but it allows for extension in more general cases than feed forward neural nets.
type Neuron interface {
Meta() Meta
Weights() *Weights
Fwd(x xmath.Vector) xmath.Vector
Bwd(dy xmath.Vector) xmath.Vector
}
```

The implementations of this interface are single operations that need to be back-tracable, in terms of errors and
gradients.

- For a forward step the cell needs to transform the given vector by applying its weights , biases etc ...
- For each backward step the cell needs to `absorb` it's part of the difference (error) from the expected output, while
  updating any related values to it s weights etc ...

The implementation for different cells are based on the following basic
concept [determining backpropagation equations](http://practicalcryptography.com/miscellaneous/machine-learning/graphically-determining-backpropagation-equations/)

### Activation Cell

### Weight Cell

### Soft Cell