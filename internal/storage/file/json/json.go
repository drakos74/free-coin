package json

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/drakos74/free-coin/internal/storage"
)

// Save saves the given json struct into the given path with the provided filename.
func Save(filePath string, fileName string, value interface{}) error {
	// check if filepath exists
	info, err := os.Stat(filePath)
	if err != nil {
		err := os.MkdirAll(filePath, os.ModePerm)
		if err != nil {
			return fmt.Errorf("could not make dir: %s: %w", filePath, err)
		}
	} else if !info.IsDir() {
		return fmt.Errorf("timePath given is not a timePath: %s", filePath)
	}

	// create the output file
	p := filepath.Join(filePath, fileName)
	f, err := os.Create(p)
	if err != nil {
		return fmt.Errorf("could not create file '%s': %w", p, err)
	}
	defer f.Close()

	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("could not save key '%+v': %w", p, err)
	}

	// write the file
	_, err = f.Write(b)
	if err != nil {
		return fmt.Errorf("could not write bytes '%+v' to file '%v' : %w", p, f, err)
	}

	return nil

}

// Load loads the payload from the given filePath and fileName.
func Load(filePath string, fileName string, value interface{}) error {

	p := filepath.Join(filePath, fileName)

	data, err := ioutil.ReadFile(p)
	if err != nil {
		return fmt.Errorf("could not read file '%s' %s: %w", p, err.Error(), storage.NotFoundErr)
	}

	err = json.Unmarshal(data, value)
	if err != nil {
		return fmt.Errorf("could not unmarshal key '%s': %w", err, storage.CouldNotLoadErr)
	}

	return nil
}
