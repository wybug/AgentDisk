package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/agentdisk/agent-disk/pkg/download_token"
)

func main() {
	secret := flag.String("secret", "", "Download token secret (required)")
	userID := flag.String("userId", "", "User ID (required)")
	fileID := flag.String("fileId", "", "File ID (required)")
	expired := flag.Bool("expired", false, "Generate an expired token")
	expireSeconds := flag.Int("expireSeconds", 300, "Token expiration in seconds")
	flag.Parse()

	if *secret == "" || *userID == "" || *fileID == "" {
		fmt.Fprintln(os.Stderr, "Usage: gen_dl_token -secret <secret> -userId <userId> -fileId <fileId> [-expired] [-expireSeconds <seconds>]")
		os.Exit(1)
	}

	if *expired {
		*expireSeconds = -1
	}

	token, err := download_token.Generate(*secret, *userID, *fileID, *expireSeconds)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(token)
}
