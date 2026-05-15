package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/agentdisk/agent-disk/pkg/jwt"
)

func main() {
	secret := flag.String("secret", "", "JWT secret (required)")
	userID := flag.String("userId", "", "User ID (required)")
	agentID := flag.String("agentId", "", "Agent ID (optional)")
	expireHours := flag.Int("expireHours", 72, "Token expiration in hours")
	flag.Parse()

	if *secret == "" || *userID == "" {
		fmt.Fprintln(os.Stderr, "Usage: gen_token -secret <secret> -userId <userId> [-agentId <agentId>] [-expireHours <hours>]")
		os.Exit(1)
	}

	token, err := jwt.GenerateToken(*secret, *userID, *agentID, *expireHours)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(token)
}
