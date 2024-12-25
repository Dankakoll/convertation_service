package main

import (
	"log"

	"github.com/joho/godotenv"
)

func init() {
	err := godotenv.Load("../config/config.env")
	if err != nil {
		log.Fatal("No conf file provided. Check link")
	}

}
