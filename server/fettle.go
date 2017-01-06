package server

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"

	"os"

	"net"

	"strconv"

	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
)

type response struct {
	Message string
}

// Instance represents a fettel server
type Instance struct {
	ID            uuid.UUID
	Name          string
	ConsulAddress url.URL
	Address       url.URL
}

// NewInstance creates a new Fettle instance
func NewInstance(name string, consulAddress url.URL, address url.URL) Instance {
	return Instance{uuid.New(), name, consulAddress, address}
}

func writeResponse(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(response{message})
}

func (ins *Instance) register(url.URL) {
	config := api.DefaultConfig()
	config.Address = ins.ConsulAddress.Host
	client, err := api.NewClient(config)
	if err != nil {
		log.Println("Cannot connect to consul:", err.Error())
	}

	checkURL := ins.Address
	checkURL.Path = "/health"
	checkURL.Query().Set("id", ins.ID.String())

	addr, portS, _ := net.SplitHostPort(ins.Address.Host)
	port, _ := strconv.ParseInt(portS, 10, 0)

	reg := &api.AgentServiceRegistration{
		ID:      ins.ID.String(),
		Name:    ins.Name + "-" + ins.ID.String(),
		Address: addr,
		Port:    int(port),
		Tags:    []string{},
		Check: &api.AgentServiceCheck{
			HTTP:                           checkURL.String(),
			Interval:                       "10s",
			DeregisterCriticalServiceAfter: "10m",
		},
	}

	client.Agent().ServiceRegister(reg)
}

func getEnv(key string, def string) string {
	value := os.Getenv(key)
	if value == "" {
		return def
	}

	return value
}

// Start fettle
func Start() {
	consulAddress, err := url.Parse(getEnv("FETTLE_CONSUL_ADDRESS", "http://0.0.0.0:8500"))
	if err != nil {
		log.Panicln("Invalid FETTLE_CONSUL_ADDRESS")
	}

	instance := NewInstance(*consulAddress)

	fettleAddress := getEnv("FETTLE_ADDRESS", "0.0.0.0")
	fettlePort := getEnv("FETTLE_PORT", "8099")

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
