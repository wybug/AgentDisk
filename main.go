package main

import (
	"log"

	"github.com/agentdisk/agent-disk/config"
	"github.com/agentdisk/agent-disk/internal/router"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	r := router.Setup(cfg)

	addr := ":" + cfg.Server.Port
	log.Printf("AgentDisk starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
