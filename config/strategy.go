package config

import "time"

type DynamicStrategy struct {
	DefaultTheme            string        `mapstructure:"DEFAULTTHEME" yaml:"defaultTheme" default:"hacker-terminal"`
	DefaultRefreshFrequency time.Duration `mapstructure:"DEFAULTREFRESHFREQUENCY" yaml:"defaultRefreshFrequency" default:"5s"`
}

type BlockingStrategy struct {
	DefaultTimeout time.Duration `mapstructure:"DEFAULTTIMEOUT" yaml:"defaultTimeout" default:"1m"`
}

type Strategy struct {
	Dynamic  DynamicStrategy
	Blocking BlockingStrategy
}

func NewStrategyConfig() Strategy {
	return Strategy{
		Dynamic:  newDynamicStrategy(),
		Blocking: newBlockingStrategy(),
	}
}

func newDynamicStrategy() DynamicStrategy {
	return DynamicStrategy{
		DefaultTheme:            "hacker-terminal",
		DefaultRefreshFrequency: 5 * time.Second,
	}
}

func newBlockingStrategy() BlockingStrategy {
	return BlockingStrategy{
		DefaultTimeout: 1 * time.Minute,
	}
}
