package types

type Instance struct {
	Name            string
	Kind            string
	Status          string
	Replicas        uint64
	DesiredReplicas uint64
	ScalingReplicas uint64
	Group           string
}
