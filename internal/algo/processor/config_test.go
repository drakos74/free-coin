package processor

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigJson(t *testing.T) {

	config := Config{
		Duration: 5,
		Order: Order{
			Name: "O2",
		},
		Notify: Notify{
			Stats: true,
		},
		Model: []Stats{
			{
				LookBack:  1,
				LookAhead: 1,
			},
		},
		Strategies: []Strategy{
			{
				Open: Open{
					Value: 100,
					Limit: 1000,
				},
				Close: Close{
					Instant: true,
					Profit: Setup{
						Min:   0.8,
						Trail: 0.15,
					},
					Loss: Setup{
						Min: 1,
					},
				},
				Name:        "numeric",
				Probability: 0.5,
				Sample:      11,
				Threshold:   6,
				Factor:      1,
			},
		},
	}

	b, err := json.Marshal(config)
	assert.NoError(t, err)
	println(fmt.Sprintf("b = %+v", string(b)))

}
