package server

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"

	"os"

	"net"

	"strconv"

	"os/exec"
	"strings"

	"bufio"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
)

type response struct {
	Message string
}

type Config struct {
	FettlePort    string
	FettleAddress string
}

func DefaultConfig() *Config {
	conf := Config{
		FettleAddress: getEnv("FETTLE_ADDRESS", "0.0.0.0"),
		FettlePort:    getEnv("FETTLE_PORT", "8099"),
	}

	return &conf
}

// Instance represents a fettel server
type Instance struct {
	ID            uuid.UUID
	Name          string
	ConsulAddress url.URL
	Address       url.URL
	Conf          *Config
}

// NewInstance creates a new Fettle instance
func NewInstance(name string, consulAddress *url.URL, address *url.URL) Instance {
	return Instance{uuid.New(), name, *consulAddress, *address, DefaultConfig()}
}

func writeResponse(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(response{message})
}

// CreateCheckURL create the health check url from the service id
func (ins *Instance) CreateCheckURL() string {
	checkURL := ins.Address
	checkURL.Path = "/health"

	query := checkURL.Query()
	query.Add("id", ins.ID.String())
	checkURL.RawQuery = query.Encode()

	return checkURL.String()
}

// RunSubprocess runs the give command and prints it's output
// to the stdout
func (ins *Instance) RunSubprocess(command string) chan error {
	stdout := make(chan bool, 1)
	stderr := make(chan bool, 1)
	result := make(chan error, 1)

	args := strings.Fields(command)

	log.Println("Starting command:", command)

	cmd := exec.Command(args[0], args[1:]...)

	go func() {
		pipe, err := cmd.StderrPipe()
		if err != nil {
			log.Panicln("Error opening stderr pipe", err)
		}

		stderr <- true

		scanner := bufio.NewScanner(pipe)

		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}

		stderr <- true
	}()

	go func() {
		pipe, err := cmd.StdoutPipe()
		if err != nil {
			log.Panicln("Error opening stdout pipe", err)
		}

		stdout <- true

		scanner := bufio.NewScanner(pipe)

		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}

		stdout <- true
	}()

	<-stderr
	<-stdout
	err := cmd.Start()
	if err != nil {
		log.Panicln("Cannot start command:", command, ", Error:", err)
	}

	go func() {
		<-stderr
		<-stdout

		result <- cmd.Wait()
	}()

	return result
}

func (ins *Instance) register() error {
	config := api.DefaultConfig()
	config.Address = ins.ConsulAddress.Host
	client, err := api.NewClient(config)

	if err != nil {
		log.Println("Cannot connect to consul:", err.Error())
	}

	addr, portS, _ := net.SplitHostPort(ins.Address.Host)
	if portS == "" {
		portS = "80"
	}
	port, _ := strconv.ParseInt(portS, 10, 0)

	reg := &api.AgentServiceRegistration{
		ID:      ins.Name + ins.ID.String(),
		Name:    ins.Name,
		Address: addr,
		Port:    int(port),
		Tags:    []string{},
		Check: &api.AgentServiceCheck{
			HTTP:                           ins.CreateCheckURL(),
			Interval:                       "10s",
			DeregisterCriticalServiceAfter: "10m",
		},
	}

	log.Println("Registering service")

	return client.Agent().ServiceRegister(reg)
}

func getEnv(key string, def string) string {
	value := os.Getenv(key)
	if value == "" {
		return def
	}

	return value
}

func getEnvRequired(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Panicln("Required configuration missing:", key)
	}

	return value
}

func getEnvRequiredURL(key string) *url.URL {
	v := getEnvRequired(key)

	ret, err := url.Parse(v)
	if err != nil {
		log.Panicln("Required configuration is not a valid URL:", key)
	}

	return ret
}

func (ins *Instance) runServer() chan error {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		id, present := r.URL.Query()["id"]
		if present {
			uuid, err := uuid.Parse(id[0])
			if err != nil || uuid != ins.ID {
				writeResponse(w, "Mismatch", 404)
			} else {
				writeResponse(w, "OK", 200)
			}
		} else {
			writeResponse(w, "Mismatch", 404)
		}
	})

	listenAddress := ins.Conf.FettleAddress + ":" + ins.Conf.FettlePort

	result := make(chan error, 1)

	go func() {
		log.Println("Listening on", listenAddress)
		err := http.ListenAndServe(listenAddress, nil)
		result <- err
	}()

	return result
}

// Start fettle
func Start() {
	consulAddress, err := url.Parse(getEnv("FETTLE_CONSUL_ADDRESS", "http://127.0.0.1:8500"))
	if err != nil {
		log.Panicln("Invalid FETTLE_CONSUL_ADDRESS")
	}

	instance := NewInstance(getEnvRequired("FETTLE_SERVICE_NAME"),
		consulAddress,
		getEnvRequiredURL("FETTLE_SERVICE_URL"))

	log.Println("Starting new fettle instance with id", instance.ID)

	err = instance.register()
	if err != nil {
		log.Println("Cannot register:", err)
	}

	select {
	case err := <-instance.RunSubprocess("ping 127.0.0.1 -n 6"):
		log.Fatalln("Supervised process exited:", err)
	case err := <-instance.runServer():
		log.Panicln("HTTP server exited:", err)
	}
}
