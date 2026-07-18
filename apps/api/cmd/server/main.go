package main

import (
	"log"

	"github.com/marcel/dungeon-master/api/internal/httpapi"
)

func main() {
	cfg := httpapi.LoadConfig()
	server := httpapi.NewServer(cfg)

	log.Printf("starting api on %s", cfg.Address())
	if err := server.Run(cfg.Address()); err != nil {
		log.Fatal(err)
	}
}
