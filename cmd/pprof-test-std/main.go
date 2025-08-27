package main

import (
	"errors"
	"log"
	"net/http"
	_ "net/http/pprof"
)

func main() {
	go func() {
		log.Println("Starting pprof server on localhost:6060")
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	stdMux := http.NewServeMux()
	stdMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("Hello, World!"))
		if err != nil {
			return
		}
	})
	log.Println("Starting web application server on localhost:8080")
	if err := http.ListenAndServe(":8080", stdMux); !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}
