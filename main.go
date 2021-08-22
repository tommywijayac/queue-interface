package main

import (
	"fmt"

	"github.com/spf13/viper"
)

func init() {
	fmt.Println("Hello from main init!")
}

type SessionKey struct {
	Auth    []byte
	Encrypt []byte
}

type Config struct {
	// Branches []BranchData

	// RoomMap     map[string]map[string]RoomData //process code -> room code -> room data
	// OrderedRoom map[string]map[int]string      //process code -> room order -> room code

	DatabaseAddr string
	DatabaseUser string
	DatabasePswd string
	DatabaseName string

	PrimaryKey   SessionKey
	SecondaryKey SessionKey
	Port         string
}

func (cfg *Config) ReadConfig() {

	viper.SetConfigFile("./config.env")
	viper.AutomaticEnv()
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Println(err.Error())
	}

	var temp string
	temp = viper.Get("PRIMARY_SESSION_KEY_AUTH").(string)
	cfg.PrimaryKey.Auth = []byte(temp)

	temp = viper.Get("PRIMARY_SESSION_KEY_ENCRYPT").(string)
	cfg.PrimaryKey.Encrypt = []byte(temp)

	temp = viper.Get("SECONDARY_SESSION_KEY_AUTH").(string)
	cfg.SecondaryKey.Auth = []byte(temp)

	temp = viper.Get("PRIMARY_SESSION_KEY_ENCRYPT").(string)
	cfg.SecondaryKey.Encrypt = []byte(temp)

	cfg.Port = viper.GetString("PORT")
	cfg.DatabaseAddr = viper.GetString("DB_ADDRESS")
	cfg.DatabaseName = viper.GetString("DB_NAME")
	cfg.DatabaseUser = viper.GetString("DB_USER")
	cfg.DatabasePswd = viper.GetString("DB_PASSWORD")
}

var AppConfig Config

func main() {
	// Load config
	AppConfig.ReadConfig()

	ReadConfig()
	Initialize()

	if AppConfig.Port == "" {
		fmt.Println("Port set to default")
		AppConfig.Port = "8080"
	}

	Run(AppConfig.Port)
}
