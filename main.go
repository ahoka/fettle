package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type response struct {
	Message string
}

func main() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Req: ", r.URL)

		id, present := r.URL.Query()["id"]

		log.Println(present)

		if present {
			log.Println("Got id: ", id[0])

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(response{"OK"})
		} else {
			log.Println("No Id present, rejecting")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(404)
			json.NewEncoder(w).Encode(response{"Invalid ID"})
		}
	})

	log.Println("Listening on 8099")
	log.Fatal(http.ListenAndServe(":8099", nil))
}
