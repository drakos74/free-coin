package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
)

const (
	// TODO : get from env
	port = 6080
	dir  = "ui/build"
)

func main() {

	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		// this should point to the react build path
		http.ServeFile(writer, request, filepath.Join(dir, request.URL.Path[1:]))
	})

	log.Println(fmt.Sprintf("Listening static on :%d...", port))
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		log.Fatal(err)
	}
}
