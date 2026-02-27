package main

import (
	"log"
	"net/http"
	"os"

	"toollab-v2/internal/run/api"
	"toollab-v2/internal/run/store"
)

func main() {
	addr := os.Getenv("TOOLLAB_V2_ADDR")
	if addr == "" {
		addr = ":8090"
	}

	st := store.New()
	srv := api.NewServer(st)

	log.Printf("toollab-v2 api listening on %s", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}
