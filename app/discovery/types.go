package discovery

const (
	LabelEnable                      = "sablier.enable"
	LabelGroup                       = "sablier.group"
	LabelGroupDefaultValue           = "default"
	LabelReplicas                    = "sablier.replicas"
	LabelReplicasDefaultValue uint64 = 1
)

type Group struct {
	Name      string
	Instances []Instance
}

type Instance struct {
	Name string
}
