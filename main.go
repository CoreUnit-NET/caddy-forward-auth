package main

import (
	"fmt"
	"log"
	"os"

	"github.com/NobleMajo/intern-auth-gateway/internal/config"
	"github.com/NobleMajo/intern-auth-gateway/internal/serve"
	"github.com/joho/godotenv"
)

var DisplayName string = "Unset"
var ShortName string = "unset"
var Version string = "?.?.?"
var Commit string = "???????"

func main() {
	_ = godotenv.Load()

	logger := log.New(os.Stdout, "", log.LstdFlags)
	appConfig := config.ParseConfig(DisplayName, ShortName, Version, Commit)

	var err error
	if appConfig.Subcommand == "serve" {
		err = serve.Run(logger, appConfig)
	} else {
		fmt.Fprintf(os.Stderr, "%s: '%s' is not a command.\nSee '%s --help'\n", os.Args[0], appConfig.Subcommand, os.Args[0])
		os.Exit(1)
	}

	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
