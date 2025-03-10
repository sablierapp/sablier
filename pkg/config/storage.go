package config

type Storage struct {
	File string `mapstructure:"FILE" yaml:"file" default:""`
}

func NewStorageConfig() Storage {
	return Storage{
		File: "",
	}
}
