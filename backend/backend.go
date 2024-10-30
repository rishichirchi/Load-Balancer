package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main(){
	port := os.Getenv("PORT")

	if port == ""{
		log.Fatal("Port is not set")
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Backend server running on port %s", port)
	})

	log.Printf("Server running on port %s\n", port)

	log.Fatal(http.ListenAndServe(":" + port, nil))

}