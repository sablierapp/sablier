package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/acouvreur/sablier/app/middleware"
	"github.com/acouvreur/sablier/config"
	"github.com/acouvreur/sablier/pkg/scaler"
	"github.com/acouvreur/sablier/pkg/storage"
	"github.com/acouvreur/sablier/pkg/tinykv"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type OnDemandRequestState struct {
	State string `json:"state"`
	Name  string `json:"name"`
}

func Start(conf config.Config) error {

	scaler, err := initScaler(conf.Provider)
	if err != nil {
		return (err)
	}
	log.Infof("using provider \"%s\"", conf.Provider.Name)

	store, err := initRuntimeStorage(scaler)
	if err != nil {
		return (err)
	}

	err = initPersistentStorage(conf.Storage, store)
	if err != nil {
		return (err)
	}

	err = initServer(conf.Server, scaler, store)
	if err != nil {
		return (err)
	}

	return nil
}

func initScaler(conf config.Provider) (scaler.Scaler, error) {

	err := conf.IsValid()
	if err != nil {
		return nil, err
	}

	switch {
	case conf.Name == "swarm":
		cli, err := client.NewClientWithOpts()
		if err != nil {
			log.Fatal(fmt.Errorf("%+v", "Could not connect to docker API"))
		}
		return &scaler.DockerSwarmScaler{
			Client: cli,
		}, nil
	case conf.Name == "docker":
		cli, err := client.NewClientWithOpts()
		if err != nil {
			log.Fatal(fmt.Errorf("%+v", "Could not connect to docker API"))
		}
		return &scaler.DockerClassicScaler{
			Client: cli,
		}, nil
	case conf.Name == "kubernetes":
		config, err := rest.InClusterConfig()
		if err != nil {
			log.Fatal(err)
		}
		client, err := kubernetes.NewForConfig(config)
		if err != nil {
			log.Fatal(err)
		}
		return scaler.NewKubernetesScaler(client), nil
	}

	return nil, fmt.Errorf("unimplemented provider %s", conf.Name)
}

func initRuntimeStorage(scaler scaler.Scaler) (tinykv.KV[OnDemandRequestState], error) {
	// TODO: Add some checks
	return tinykv.New(time.Second*20, func(key string, _ OnDemandRequestState) {
		// Auto scale down after timeout
		err := scaler.ScaleDown(key)

		if err != nil {
			log.Warnf("error scaling down %s: %s", key, err.Error())
		}
	}), nil
}

func initPersistentStorage(config config.Storage, store tinykv.KV[OnDemandRequestState]) error {
	if len(config.File) > 0 {
		file, err := os.OpenFile(config.File, os.O_RDWR|os.O_CREATE, 0755)

		if err != nil {
			return err
		}

		// TODO: Add data check
		json.NewDecoder(file).Decode(store)
		storage.New(file, time.Second*5, store)
		log.Infof("initialized storage to %s", config.File)
	} else {
		log.Infof("no storage configuration provided. all states will be lost upon exit")
	}
	return nil
}

func initServer(conf config.Server, scaler scaler.Scaler, store tinykv.KV[OnDemandRequestState]) error {
	r := gin.Default()

	r.Use(middleware.Logger(log.New()), gin.Recovery())

	base := r.Group(conf.BasePath)
	{
		base.GET("/", onDemand(scaler, store))
	}

	r.Run(fmt.Sprintf(":%d", conf.Port))
	return nil
}

func onDemand(scaler scaler.Scaler, store tinykv.KV[OnDemandRequestState]) func(c *gin.Context) {
	return func(c *gin.Context) {

		name := c.Query("name")
		to := c.Query("timeout")

		if name == "" || to == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name or timeout empty"})
			return
		}

		timeout, err := time.ParseDuration(to)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
			}
		}

		// 2. Store the updated state
		store.Put(name, requestState, tinykv.ExpiresAfter(timeout))

		// 3. Serve depending on the current state
		switch requestState.State {
		case "starting":
			c.JSON(http.StatusAccepted, gin.H{"state": requestState.State})
		case "started":
			c.JSON(http.StatusCreated, gin.H{"state": requestState.State})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Errorf("unknown state %s", requestState.State)})
		}
	}
}
