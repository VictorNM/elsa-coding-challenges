package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/victornm/equiz/internal/config"
	"github.com/victornm/equiz/internal/server"
)

func main() {
	c, err := loadConfig()
	if err != nil {
		log.Fatalf("Load config failed: %v", err)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGTERM, os.Interrupt)

	s, err := server.Init(c)
	if err != nil {
		log.Fatalf("Init server failed: %v", err)
	}

	go s.Start()

	<-shutdown
	s.Shutdown()
}

func loadConfig() (server.Config, error) {
	var c server.Config

	p := os.Getenv("CONFIG_PATH")
	if p == "" {
		return c, fmt.Errorf("CONFIG_PATH not set")
	}

	if err := config.Load(p, &c); err != nil {
		return c, fmt.Errorf("load config: %w", err)
	}

	return c, nil
}
