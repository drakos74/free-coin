package json

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

// Capture captures the given payload as a json in the given file
// NOTE : the file needs to exist. This is to avoid unexpected behaviour.
func Capture(payload interface{}, filename string) error {

	bb, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("could not marshal payload: %w", err)
	}

	err = ioutil.WriteFile(filename, bb, 0644)
	if err != nil {
		return fmt.Errorf("could not write file: %w", err)
	}

	return nil

}
