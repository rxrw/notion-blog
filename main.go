package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"notion2md/internal"
	translator "notion2md/pkg"

	"github.com/itzg/go-flagsfiller"
	"github.com/joho/godotenv"
)

var config translator.BlogConfig

func parseJSONConfig() error {
	wkspc := "."
	if os.Getenv("GITHUB_WORKSPACE") == "" {
		wkspc = os.Getenv("GITHUB_WORKSPACE")
	}
	configPath := wkspc + "/notionblog.config.json"
	log.Printf("Reading config from %s", configPath)
	content, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatal("error reading config file: ", err)
		return err
	}
	json.Unmarshal(content, &config)
	return nil
}

func parseFlagsConfig() {
	// create a FlagSetFiller
	filler := flagsfiller.New()
	// fill and map struct fields to flags
	err := filler.Fill(flag.CommandLine, &config)
	if err != nil {
		log.Fatal(err)
	}

	// parse command-line like usual
	flag.Parse()
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file provided")
	}

	err = parseJSONConfig()
	if err != nil {
		parseFlagsConfig()
	}

	err = internal.ParseAndGenerate(config)
	if err != nil {
		log.Println(err)
	}
}
