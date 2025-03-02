package discovery

const (
	LabelEnable            = "sablier.enable"
	LabelGroup             = "sablier.group"
	LabelGroupDefaultValue = "default"
)

type Group struct {
	Name      string
	Instances []Instance
}

type Instance struct {
	Name string
}
