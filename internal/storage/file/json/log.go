package json

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/storage"
)

const (
	filename = "%d.events.log"
)

type Logger struct {
	path string
}

func NewLogger(folder string) *Logger {
	return &Logger{path: folder}
}

func (l *Logger) filePath(k storage.K) string {
	return path.Join(storage.DefaultDir, storage.RegistryDir, l.path, k.Pair, k.Label)
}

func (l *Logger) Store(k storage.Key, value interface{}) error {

	filePath := l.filePath(storage.K{
		Pair:  k.Pair,
		Label: k.Label,
	})

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

	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("could not encode value '%+v': %w", value, err)
	}
	f, err := os.OpenFile(path.Join(filePath, fmt.Sprintf(filename, k.Hash)), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("could not open log file: %w", err)
	}

	defer f.Close()

	if _, err = f.Write(append(b, []byte("\n")...)); err != nil {
		return fmt.Errorf("could not write log file for  '%+v': %w", k, err)
	}
	return nil
}

func (l Logger) Load(k storage.Key, value interface{}) error {

	switch value.(type) {
	case *string:
	default:
		return fmt.Errorf("only string references are allowed for this: %v", value)
	}

	fileName := path.Join(l.filePath(storage.K{
		Pair:  k.Pair,
		Label: k.Label,
	}), fmt.Sprintf(filename, k.Hash))

	b, err := ioutil.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("could not read file '%s': %w", fileName, err)
	}

	vv := reflect.Indirect(reflect.ValueOf(value))
	v := string(b)
	vv.Set(reflect.ValueOf(v))

	return nil

}

type Registry struct {
	hash   int64
	logger *Logger
	root   string
}

func NewEventRegistry(path string) *Registry {
	return &Registry{
		hash:   time.Now().Unix(),
		logger: NewLogger(path),
		root:   path,
	}
}

// EventRegistry creates a new registry generator
func EventRegistry(parent string) storage.EventRegistry {
	return func(p string) (storage.Registry, error) {
		if p == "" {
			return NewEventRegistry(parent), nil
		}
		return NewEventRegistry(path.Join(parent, p)), nil
	}
}

func (e *Registry) WithHash(h int64) *Registry {
	e.hash = h
	return e
}

func (e *Registry) Root() string {
	return e.root
}

func (e *Registry) Add(key storage.K, value interface{}) error {
	k := storage.Key{
		Hash:  e.hash,
		Pair:  key.Pair,
		Label: key.Label,
	}
	return e.logger.Store(k, value)
}

// Get appends the values to the given slice
// Not sure it s worth all the effort and abstraction ... but wtf
func (e *Registry) GetAll(key storage.K, values interface{}) error {

	if reflect.Indirect(reflect.ValueOf(values)).Kind() != reflect.Slice {
		return fmt.Errorf("only accepting slices as placeholder for the results")
	}

	var instance interface{}
	t := reflect.Indirect(reflect.ValueOf(values)).Index(0).Type()
	instance = reflect.New(t).Interface()

	filePath := e.logger.filePath(key)
	//pInstance := reflect.ValueOf(instance).Pointer()
	// find our file ... which hash to choose (?)
	elemSlice := reflect.MakeSlice(reflect.SliceOf(t), 0, 10)
	err := filepath.Walk(filePath, func(path string, info os.FileInfo, err error) error {
		if info != nil && !info.IsDir() {
			// just load it anyway
			n := info.Name()
			h, err := strconv.ParseInt(strings.Split(n, ".")[0], 10, 64)
			if err != nil {
				return fmt.Errorf("non-numberic path '%s' found for hash: %w", path, err)
			}
			var ss string
			err = e.logger.Load(storage.Key{
				Hash:  h,
				Pair:  key.Pair,
				Label: key.Label,
			}, &ss)
			if err != nil {
				return fmt.Errorf("could not load key '%+v': %w", key, err)
			}
			c := 0
			for _, s := range strings.Split(ss, "\n") {
				if s == "" {
					continue
				}
				err = json.Unmarshal([]byte(s), instance)
				if err != nil {
					return fmt.Errorf("could not decode event value '%+v': %w", s, err)
				}
				ev := reflect.Indirect(reflect.ValueOf(instance))
				elemSlice = reflect.Append(elemSlice, ev)
				c++
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("could not get events: %w", err)
	}

	// do teh last pointer assignment ... and ... done :)
	vv := reflect.Indirect(reflect.ValueOf(values))
	vv.Set(elemSlice)

	return nil
}
