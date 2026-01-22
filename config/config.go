package config

import "github.com/spf13/viper"

type Config struct {
	Slack struct {
		Webhook string `mapstructure:"webhook"`
	} `mapstructure:"slack"`

	HE struct {
		Eth  string `mapstructure:"eth"`
		Tron string `mapstructure:"tron"`
	} `mapstructure:"he"`

	WatchList []struct {
		Chain     string   `mapstructure:"chain"`
		Endpoint  string   `mapstructure:"endpoint"`
		Contracts []string `mapstructure:"contracts"`
	} `mapstructure:"track_list"`
}

var C Config

func init() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}

	if err := viper.Unmarshal(&C); err != nil {
		panic(err)
	}
}
