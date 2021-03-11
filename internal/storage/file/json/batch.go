package json

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

type BlobStorage struct {
	path  string
	table string
	shard string
	debug bool
}

func BlobShard(table string) storage.Shard {
	return func(shard string) (storage.Persistence, error) {
		return NewJsonBlob(table, shard, false), nil
	}
}

func (s BlobStorage) Store(k storage.Key, value interface{}) error {
	p := filepath.Join(s.path, s.table, s.shard)
	err := Save(p, k.Path(), value)
	if err != nil && s.debug {
		log.Info().Str("path", p).Str("file", k.Path()).Str("file", p).Msg("stored json file")
	}
	return err
}

func (s BlobStorage) Load(k storage.Key, value interface{}) error {
	return Load(filepath.Join(s.path, s.table, s.shard), k.Path(), value)
}

// table has the same schema
// shard is a logical split
func NewJsonBlob(table, shard string, debug bool) *BlobStorage {
	return &BlobStorage{
		table: table,
		shard: shard,
		path:  storage.DefaultDir,
		debug: debug,
	}
}

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
	f, err := os.Create(fmt.Sprintf("%s.json", p))
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
	//
	//path, err := os.Getwd()
	//if err != nil {
	//	return err
	//}
	//fmt.Println(path)
	//_, err = os.Stat("file-storage")
	//fmt.Println(fmt.Sprintf("err = %+v", err))
	//

	p := filepath.Join(filePath, fileName)

	data, err := ioutil.ReadFile(fmt.Sprintf("%s.json", p))
	var num int64
	if err != nil {
		// TODO : temporary fix ... check with prefix
		err = filepath.Walk(filePath, func(path string, info os.FileInfo, err error) error {
			if info != nil && !info.IsDir() {
				if strings.HasPrefix(info.Name(), fileName) {
					atomic.AddInt64(&num, 1)
					fileData, err := ioutil.ReadFile(path)
					if err != nil {
						return err
					}
					data = fileData
					return nil
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("could not read file '%s' %s: %w", p, err.Error(), storage.NotFoundErr)
		}
	} else {
		num = 1
	}

	if len(data) == 0 {
		log.Info().Str("path", filePath).Str("file", fileName).Msg("not found")
	}

	if num > 1 {
		return fmt.Errorf("multiple sources found")
	}

	err = json.Unmarshal(data, value)
	if err != nil {
		return fmt.Errorf("could not unmarshal key for '%s': '%v': %w", fileName, err, storage.CouldNotLoadErr)
	}

	return nil
}
