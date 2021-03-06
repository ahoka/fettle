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

	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
	"github.com/jinzhu/configor"
)

type response struct {
	Message string
}

// Config holds fettle's configuration
type Config struct {
	Fettle struct {
		Port    int    `default:"8099"`
		Address string `default:"0.0.0.0"`
	}

	Consul struct {
		Address string `default:"http://127.0.0.1:8500"`
		Health  struct {
			Interval   string `default:"10s"`
			Deregister string `default:"10m"`
			Address    string
		}
		Interval string `default:"30s"`
		Tags     []string
	}

	Service struct {
		Name    string `required:"true"`
		Address string `required:"true"`
	}

	Supervisor []struct {
		Name    string `required:"true"`
		Command string `required:"true"`
	}
}

// ConsulURL returns the URL for consul
func (ins *Instance) ConsulURL() *url.URL {
	url, err := url.Parse(ins.Conf.Consul.Address)
	if err != nil {
		log.Panicln("ConsulAddress is invalid:", err)
	}

	return url
}

// ServiceURL returns the public URL for the servicv
func (ins *Instance) ServiceURL() *url.URL {
	url, err := url.Parse(ins.Conf.Service.Address)
	if err != nil {
		log.Panicln("ServiceAddress is invalid:", err)
	}

	return url
}

// Instance represents a fettle server
type Instance struct {
	ID                uuid.UUID
	Subprocesses      []Subprocess
	SubprocessChannel chan Subprocess
	Conf              *Config
}

// NewInstance creates a new Fettle instance
func NewInstance() Instance {
	conf := Config{}

	os.Setenv("CONFIGOR_ENV_PREFIX", "FETTLE")
	err := configor.Load(&conf, "fettle.yml")
	if err != nil {
		log.Fatalln("Configuration error:", err)
	}

	log.Println("Fettle Port:", conf.Fettle.Port)
	log.Println("Fettle Address:", conf.Fettle.Address)
	log.Println("Consul Address:", conf.Consul.Address)
	log.Println("Service Name:", conf.Service.Address)
	log.Println("Service Address:", conf.Service.Address)

	return Instance{uuid.New(), make([]Subprocess, 0, 32), make(chan Subprocess, 1), &conf}
}

func writeResponse(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(response{message})
}

// CreateCheckURL create the health check url from the service id
func (ins *Instance) CreateCheckURL() string {
	var checkURL url.URL

	addr := ins.Conf.Consul.Health.Address
	if addr == "" {
		checkURL = *ins.ServiceURL()
	} else {
		url, err := url.Parse(addr)
		if err != nil {
			log.Panicln("Health URL is invalid:", err)
		}
		checkURL = *url
	}

	checkURL.Path = "/health"

	query := checkURL.Query()
	query.Add("id", ins.ID.String())
	checkURL.RawQuery = query.Encode()

	return checkURL.String()
}

type Subprocess struct {
	Name    string
	Command string
	Error   error
}

// RunSubprocess runs the give command and prints it's output
// to the stdout
func (ins *Instance) RunSubprocess(name string, command string) {
	stdout := make(chan bool, 1)
	stderr := make(chan bool, 1)

	args := strings.Fields(command)

	log.Println("Starting command:", command)
	proc := Subprocess{Name: name, Command: command}
	ins.Subprocesses = append(ins.Subprocesses, proc)

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

		err := cmd.Wait()
		proc.Error = err

		ins.SubprocessChannel <- proc
	}()
}

// Register registers the service to consul
func (ins *Instance) Register() {
	addr, portS, _ := net.SplitHostPort(ins.ServiceURL().Host)
	if portS == "" {
		portS = "80"
	}
	port, _ := strconv.ParseInt(portS, 10, 0)
	config := api.DefaultConfig()
	config.Address = ins.ConsulURL().Host

	interval, err := time.ParseDuration(ins.Conf.Consul.Interval)
	if err != nil {
		log.Panicln("Invalid Consul interval:", err)
	}

	for {
		client, err := api.NewClient(config)

		if err != nil {
			log.Println("Cannot connect to Consul:", err)
		} else {
			reg := &api.AgentServiceRegistration{
				ID:      ins.Conf.Service.Name + "-" + ins.ID.String(),
				Name:    ins.Conf.Service.Name,
				Address: addr,
				Port:    int(port),
				Tags:    ins.Conf.Consul.Tags,
				Check: &api.AgentServiceCheck{
					HTTP:                           ins.CreateCheckURL(),
					Interval:                       ins.Conf.Consul.Health.Interval,
					DeregisterCriticalServiceAfter: ins.Conf.Consul.Health.Deregister,
				},
			}

			err = client.Agent().ServiceRegister(reg)
			if err != nil {
				log.Println("Cannot register to Consul:", err)
			}
		}

		time.Sleep(interval)
	}
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

func (ins *Instance) RunServer() chan error {
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

	listenAddress := ins.Conf.Fettle.Address + ":" + strconv.Itoa(ins.Conf.Fettle.Port)

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
	log.Println("Fettle starting up...")
	log.Println("Using environment:", configor.ENV())

	instance := NewInstance()

	log.Println("Starting new fettle instance with id", instance.ID)

	for _, sup := range instance.Conf.Supervisor {
		instance.RunSubprocess(sup.Name, sup.Command)
	}

	servChan := instance.RunServer()
	instance.Register()

	select {
	case proc := <-instance.SubprocessChannel:
		log.Println("Supervised process exited!")
		log.Println("Name:", proc.Name)
		log.Println("Command:", proc.Command)
		log.Fatalln("Error:", proc.Error)
	case err := <-servChan:
		log.Panicln("HTTP server exited:", err)
	}
}
