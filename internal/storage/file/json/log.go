package json

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/drakos74/free-coin/internal/storage"
)

const parent = "events"

type Logger struct {
	path string
}

func NewLogger(path string) *Logger {
	return &Logger{path: path}
}

func (l *Logger) filePath(k storage.Key) string {
	return path.Join(storage.DefaultDir, parent, l.path, k.Pair, k.Label)
}

func (l *Logger) Store(k storage.Key, value interface{}) error {

	filePath := l.filePath(k)
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

	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("could not encode value '%+v': %w", value, err)
	}
	f, err := os.OpenFile(path.Join(filePath, fmt.Sprintf("%d.events.log", k.Hash)), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("could not open log file: %w", err)
	}

	defer f.Close()

	if _, err = f.Write(b); err != nil {
		return fmt.Errorf("could not write log file for  '%+v': %w", k, err)
	}
	return nil
}

func (l Logger) Load(k storage.Key, value interface{}) error {
	panic("implement me")
}

type Events struct {
	hash   int64
	logger *Logger
}

func NewEventStore(path string) *Events {
	return &Events{
		hash:   time.Now().Unix(),
		logger: NewLogger(path),
	}
}

func (e *Events) Put(key storage.K, value interface{}) error {
	k := storage.Key{
		Hash:  e.hash,
		Pair:  key.Pair,
		Label: key.Label,
	}
	return e.logger.Store(k, value)
}

func (e *Events) Get(key storage.K, value interface{}) error {

	panic("implement me")
}

// Void is a dummy event logger which ignores all calls
type Void struct {
}

func VoidEvents() *Void {
	return &Void{}
}

func (v Void) Put(key storage.K, value interface{}) error {
	return nil
}

func (v Void) Get(key storage.K, value interface{}) error {
	return nil
}
