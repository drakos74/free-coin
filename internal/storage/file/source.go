package file

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"
)

// Key represent a store key reference for the badger store.
type Key struct {
	Pair  string    `json:"pair"`
	Since int64     `json:"since"`
	From  time.Time `json:"from"`
	To    time.Time `json:"to"`
	Count int       `json:"count"`
}

// Save saves the key to the given timePath
func (k *Key) Save(filePath string) error {

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
	path := timePath(filePath, k.From)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("could not create file '%s': %w", path, err)
	}
	defer f.Close()

	b, err := json.Marshal(*k)
	if err != nil {
		return fmt.Errorf("could not save key '%+v': %w", *k, err)
	}

	// write the file
	_, err = f.Write(b)
	if err != nil {
		return fmt.Errorf("could not write bytes '%+v' to file '%v' : %w", *k, f, err)
	}

	println(fmt.Sprintf("done saving %s", f.Name()))
	return nil

}

// Load loads the key for the given file.
func (k *Key) Load(fileName string) error {

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("could not read file '%s': %w", fileName, err)
	}

	err = json.Unmarshal(data, k)
	if err != nil {
		return fmt.Errorf("could not unmarshal key: %w", err)
	}

	return nil
}

func timePath(filename string, from time.Time) string {
	return fmt.Sprintf("%s/%v_%v_%v.json", filename, from.Year(), from.Month(), from.Day())
}
