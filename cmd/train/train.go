package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/drakos74/free-coin/internal/algo/processor/ml"

	"github.com/drakos74/free-coin/internal/model"

	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func main() {

	files := make([]string, 0)

	err := filepath.Walk(fmt.Sprintf("%s/%s/BTC/2018_2019", storage.DefaultDir, storage.HistoryDir), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			panic(fmt.Sprintf("could not read directory : %+v", err))
			return err
		}
		if !info.IsDir() && !strings.HasPrefix(info.Name(), ".") {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		panic(fmt.Errorf("error during directory reading: %+v", err))
	}

	sort.Strings(files)
	fmt.Printf("files = %+v\n", files)

	in := make(chan *model.TradeSignal)
	out := make(chan *model.TradeSignal)

	proc, err := ml.ReProcessor()
	if err != nil {
		panic(fmt.Sprintf("could not set up porocessor : %+v", err))
	}

	go func() {
		for _, path := range files {
			// it s a file
			data, err := os.ReadFile(path)
			if err != nil {
				panic(fmt.Sprintf("could not read file : %+v", err))
			}

			trades := make([]model.TradeSignal, 0)
			err = json.Unmarshal(data, &trades)
			if err != nil {
				panic(fmt.Sprintf("could not unmarshall trades : %+v", err))
			}

			fmt.Printf("trades = %s %+v\n", path, len(trades))

			go proc(in, out)

			for _, trade := range trades {
				in <- &trade
			}
		}
	}()

	for range out {

	}

}
