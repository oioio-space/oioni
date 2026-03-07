package main

import (
	"fmt"
	"log"
	"time"
)

func main() {
	log.SetFlags(0)
	for {
		fmt.Println("Hello from Pi Zero 2W - oioio!")
		time.Sleep(10 * time.Second)
	}
}
