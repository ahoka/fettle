package server

import (
	"encoding/json"
	"log"
	"net/http"

	"os"

	"github.com/google/uuid"
)

type response struct {
	Message string
}

type Instance struct {
	ID uuid.UUID
}

func NewInstance() Instance {
	return Instance{uuid.New()}
}

func writeResponse(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(response{message})
}

// Start fettle
func Start() {
	instance := NewInstance()

	fettleAddress := os.Getenv("FETTLE_ADDRESS")
	if fettleAddress == "" {
		fettleAddress = "0.0.0.0"
	}

	fettlePort := os.Getenv("FETTLE_PORT")
	if fettlePort == "" {
		fettlePort = "8099"
	}

	log.Println("Starting new fettle instance with id", instance.ID)

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Req: ", r.URL)

		id, present := r.URL.Query()["id"]
		if present {
			log.Println("Got id: ", id[0])

			uuid, err := uuid.Parse(id[0])
			if err != nil || uuid != instance.ID {
				log.Println("Invalid Id, rejecting")
				writeResponse(w, "Mismatch", 404)
			} else {
				writeResponse(w, "OK", 200)
			}
		} else {
			log.Println("No Id present, rejecting")
			writeResponse(w, "Mismatch", 404)
		}
	})

	listenAddress := fettleAddress + ":" + fettlePort
	log.Println("Listening on", listenAddress)
	log.Fatal(http.ListenAndServe(listenAddress, nil))
}
