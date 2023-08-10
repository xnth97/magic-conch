package main

import (
	"encoding/json"
	"os"
)

func main() {
	configFile, err := os.Open("config.json")
	if err != nil {
		panic(err)
	}
	defer configFile.Close()

	var config Config
	jsonParser := json.NewDecoder(configFile)
	if err = jsonParser.Decode(&config); err != nil {
		panic(err)
	}

	b := NewBot(config)
	b.Start()
}
