package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/acouvreur/tinykv"
	"github.com/acouvreur/sablier/pkg/scaler"
	"github.com/acouvreur/sablier/pkg/storage"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type OnDemandRequestState struct {
	State string `json:"state"`
	Name  string `json:"name"`
}

func main() {

	swarmMode := flag.Bool("swarmMode", true, "Enable swarm mode")
	kubernetesMode := flag.Bool("kubernetesMode", false, "Enable Kubernetes mode")
	storagePath := flag.String("storagePath", "", "Enable persistent storage")

	flag.Parse()

	dockerScaler := getDockerScaler(*swarmMode, *kubernetesMode)

	store := tinykv.New[OnDemandRequestState](time.Second*20, func(key string, _ OnDemandRequestState) {
		// Auto scale down after timeout
		err := dockerScaler.ScaleDown(key)

		if err != nil {
			log.Warnf("error scaling down %s: %s", key, err.Error())
		}
	})

	if storagePath != nil && len(*storagePath) > 0 {
		file, err := os.OpenFile(*storagePath, os.O_RDWR|os.O_CREATE, 0755)

		if err != nil {
			panic(err)
		}

		json.NewDecoder(file).Decode(store)
		storage.New(file, time.Second*5, store)
	}

	fmt.Printf("Server listening on port 10000, swarmMode: %t, kubernetesMode: %t\n", *swarmMode, *kubernetesMode)
	http.HandleFunc("/", onDemand(dockerScaler, store))
	log.Fatal(http.ListenAndServe(":10000", nil))
}

func getDockerScaler(swarmMode, kubernetesMode bool) scaler.Scaler {
	cliMode := !swarmMode && !kubernetesMode

	switch {
	case swarmMode:
		cli, err := client.NewClientWithOpts()
		if err != nil {
			log.Fatal(fmt.Errorf("%+v", "Could not connect to docker API"))
		}
		return &scaler.DockerSwarmScaler{
			Client: cli,
		}
	case cliMode:
		cli, err := client.NewClientWithOpts()
		if err != nil {
			log.Fatal(fmt.Errorf("%+v", "Could not connect to docker API"))
		}
		return &scaler.DockerClassicScaler{
			Client: cli,
		}
	case kubernetesMode:
		config, err := rest.InClusterConfig()
		if err != nil {
			log.Fatal(err)
		}
		client, err := kubernetes.NewForConfig(config)
		if err != nil {
			log.Fatal(err)
		}
		return scaler.NewKubernetesScaler(client)
	}

	panic("invalid mode")
}

func onDemand(scaler scaler.Scaler, store tinykv.KV[OnDemandRequestState]) func(w http.ResponseWriter, r *http.Request) {
	return func(rw http.ResponseWriter, r *http.Request) {

		name, err := getParam(r.URL.Query(), "name")

		if err != nil {
			ServeHTTPInternalError(rw, err)
			return
		}

		to, err := getParam(r.URL.Query(), "timeout")

		if err != nil {
			ServeHTTPInternalError(rw, err)
			return
		}

		timeout, err := time.ParseDuration(to)

		if err != nil {
			ServeHTTPInternalError(rw, err)
			return
		}

		requestState, exists := store.Get(name)

		// 1. Check against the current state
		if !exists || requestState.State != "started" {
			if scaler.IsUp(name) {
				requestState = OnDemandRequestState{
					State: "started",
					Name:  name,
				}
			} else {
				requestState = OnDemandRequestState{
					State: "starting",
					Name:  name,
				}
				err := scaler.ScaleUp(name)

				if err != nil {
					ServeHTTPInternalError(rw, err)
					return
				}
			}
		}

		// 2. Store the updated state
		store.Put(name, requestState, tinykv.ExpiresAfter(timeout))

		// 3. Serve depending on the current state
		switch requestState.State {
		case "starting":
			ServeHTTPRequestState(rw, requestState)
		case "started":
			ServeHTTPRequestState(rw, requestState)
		default:
			ServeHTTPInternalError(rw, fmt.Errorf("unknown state %s", requestState.State))
		}
	}
}

func getParam(queryParams url.Values, paramName string) (string, error) {
	if queryParams[paramName] == nil {
		return "", fmt.Errorf("%s is required", paramName)
	}
	return queryParams[paramName][0], nil
}

func ServeHTTPInternalError(rw http.ResponseWriter, err error) {
	rw.WriteHeader(http.StatusInternalServerError)
	rw.Write([]byte(err.Error()))
}

func ServeHTTPRequestState(rw http.ResponseWriter, requestState OnDemandRequestState) {
	rw.Header().Set("Content-Type", "text/plain")
	if requestState.State == "started" {
		rw.WriteHeader(http.StatusCreated)
	} else {
		rw.WriteHeader(http.StatusAccepted)
	}
	rw.Write([]byte(requestState.State))
}
