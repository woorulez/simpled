package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
}

func main() {
	port := flag.Int("p", 11011, "port")
	dir := flag.String("d", ".", "directory")
	flag.Parse()

	h, err := Handler(*dir)
	if err != nil {
		log.Fatal(fmt.Printf("%v", err))
	}

	http.Handle("/", h)

	log.Printf("serving %s on port %d", *dir, *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}
