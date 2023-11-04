package session

import "encoding/json"

type InstanceStatus string

const (
	InstanceStarting InstanceStatus = "starting"
	InstanceRunning  InstanceStatus = "running"
	InstanceError    InstanceStatus = "error"
)

// Instance holds the data representing an instance status
type Instance struct {
	// The Name of the targeted container, service, deployment
	// of which the state is being represented
	Name   string
	Status InstanceStatus
	Error  error
}

func (i Instance) MarshalJSON() ([]byte, error) {
	var err string
	if i.Error != nil {
		err = i.Error.Error()
	} else {
		err = ""
	}
	return json.Marshal(struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}{
		Name:   i.Name,
		Status: string(i.Status),
		Error:  err,
	})
}
