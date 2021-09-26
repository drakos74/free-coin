# Processors

## Stats

Stats is the basic stats analysis on the previous price moves.

- splits the price movements in `x` intervals of size `t`.
- maps the change ratio (`price_diff / price`) to a range of discreet values (rounded logarithm).
- predicts the next `k` intervals based on the previous `l` ones, using a hidden markov model.
