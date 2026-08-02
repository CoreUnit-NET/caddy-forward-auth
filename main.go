package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/CoreUnit-NET/caddy-forward-auth/internal/config"
	"github.com/CoreUnit-NET/caddy-forward-auth/internal/serve"
	"github.com/joho/godotenv"
)

var DisplayName string = "Unset"
var ShortName string = "unset"
var Version string = "?.?.?"
var Commit string = "???????"

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("warning: .env: %v", err)
	}

	logger := log.New(os.Stdout, "", log.LstdFlags)
	appConfig, err := config.ParseConfig(DisplayName, ShortName)
	if errors.Is(err, config.ErrHelpRequested) {
		os.Exit(0)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if appConfig.ShowVersion {
		fmt.Println(DisplayName + " version " + Version + ", build " + Commit)
		os.Exit(0)
	}

	if err := serve.Run(logger, ShortName, appConfig); err != nil {
		log.Fatalf("error: %v", err)
	}
}
