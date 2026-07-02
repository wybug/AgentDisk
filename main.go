package main

import (
	"log"

	"github.com/agentdisk/agent-disk/config"
	"github.com/agentdisk/agent-disk/internal/router"
)

const configPath = "config.yaml"

func main() {
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	r, featureReg, err := router.Setup(cfg, configPath)
	if err != nil {
		log.Fatalf("failed to setup router: %v", err)
	}

	addr := ":" + cfg.Server.Port
	log.Printf("AgentDisk starting on %s", addr)
	if err := r.Run(addr); err != nil {
		// Best-effort cleanup before exit. The process is going down anyway;
		// an error here is informational, not actionable.
		_ = featureReg.Close()
		log.Fatalf("failed to start server: %v", err)
	}
	_ = featureReg.Close()
}
