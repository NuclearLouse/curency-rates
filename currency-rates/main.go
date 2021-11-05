package main

import (
	"currency-rates/internal/service"
	"flag"
	"log"
)

func main() {
	info := flag.Bool("v", false, "will display the version of the program")
	flag.Parse()
	if *info {
		service.Version()
		return
	}
	srv, err := service.New()
	if err != nil {
		log.Fatalln("Init service:", err)
	}
	srv.Start()
}
