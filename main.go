package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/CoreUnit-NET/intern-auth-gateway/internal/config"
	"github.com/CoreUnit-NET/intern-auth-gateway/internal/serve"
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
	appConfig := config.ParseConfig(DisplayName, ShortName, Version, Commit)

	var err error
	if appConfig.Subcommand == "serve" {
		err = serve.Run(logger, ShortName, appConfig)
	} else {
		fmt.Fprintf(os.Stderr, "%s: '%s' is not a command.\nSee '%s --help'\n", os.Args[0], appConfig.Subcommand, os.Args[0])
		os.Exit(1)
	}

	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
