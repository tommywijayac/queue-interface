package main

import (
	"fmt"
	"regexp"

	"github.com/spf13/viper"
)

type SessionKey struct {
	Auth    []byte
	Encrypt []byte
}

type RoomData struct {
	Name  string `mapstructure:"name"`
	Code  string `mapstructure:"code"`
	Order int    `mapstructure:"order"`
}

type BranchData struct {
	Name     string `mapstructure:"name"`
	Code     string `mapstructure:"code"`
	ID       string `mapstructure:"id"`
	Password string `mapstructure:"password"`
}

type Config struct {
	Branches []BranchData

	RoomMap     map[string]map[string]RoomData //process code -> room code -> room data
	OrderedRoom map[string]map[int]string      //process code -> room order -> room code

	DatabaseAddr string
	DatabaseUser string
	DatabasePswd string
	DatabaseName string

	PrimaryKey   SessionKey
	SecondaryKey SessionKey
	Port         string
}

func (cfg *Config) readConfig() {
	viper.SetConfigFile("./config.env")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("ER002: Fatal error - config file: %s", err.Error()))
	}

	readEnvByteConfig("PRIMARY_SESSION_KEY_AUTH", &cfg.PrimaryKey.Auth, []byte("super-secret-key-auth-first"))
	readEnvByteConfig("PRIMARY_SESSION_KEY_ENCRYPT", &cfg.PrimaryKey.Encrypt, []byte("super-secret-key-encrypt-first"))
	readEnvByteConfig("SECONDARY_SESSION_KEY_AUTH", &cfg.SecondaryKey.Auth, []byte("super-secret-key-auth-second"))
	readEnvByteConfig("SECONDARY_SESSION_KEY_ENCRYPT", &cfg.SecondaryKey.Encrypt, []byte("super-secret-key-encrypt-second"))

	readEnvStringConfig("PORT", &cfg.Port, "8080")
	readEnvStringConfig("DB_ADDRESS", &cfg.DatabaseAddr, "127.0.0.1:3030")
	readEnvStringConfig("DB_NAME", &cfg.DatabaseName, "kmn_queue")
	readEnvStringConfig("DB_USER", &cfg.DatabaseUser, "root")
	readEnvStringConfig("DB_PASSWORD", &cfg.DatabasePswd, "")

	// Read configuration file
	viper.SetConfigFile("./config.json")
	err = viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("ER002: Fatal error - config file: %s", err.Error()))
	}

	// Read branch configuration
	err = viper.UnmarshalKey("branch", &cfg.Branches)
	if err != nil {
		panic(fmt.Errorf("ER003: Fatal error - reading config file: %s", err.Error()))
	}
	if len(cfg.Branches) == 0 {
		panic(fmt.Errorf("ER005: Fatal error - no Branch endpoint defined"))
	}

	// Read room configuration
	cfg.RoomMap = make(map[string]map[string]RoomData)
	cfg.RoomMap["opr"] = make(map[string]RoomData)
	cfg.RoomMap["pol"] = make(map[string]RoomData)

	cfg.OrderedRoom = make(map[string]map[int]string)
	cfg.OrderedRoom["opr"] = make(map[int]string)
	cfg.OrderedRoom["pol"] = make(map[int]string)

	cfg.readRoomConfig("opr")
	cfg.readRoomConfig("pol")
}

func readEnvByteConfig(key string, dest *[]byte, default_value []byte) {
	if temp := viper.Get(key); temp != nil {
		*dest = []byte(temp.(string))
	} else {
		*dest = default_value
		fmt.Printf("%v is set with default value.\n", key)
	}
}

func readEnvStringConfig(key string, dest *string, default_value string) {
	if temp := viper.GetString(key); temp != "" {
		*dest = temp
	} else {
		*dest = default_value
		fmt.Printf("%v is set with default value.\n", key)
	}
}

// Helper function to simplify room config assignment for each process
func (cfg *Config) readRoomConfig(process string) {
	var rooms []RoomData
	var key string

	key = fmt.Sprintf("process.%s.room", process)
	err := viper.UnmarshalKey(key, &rooms)
	if err != nil {
		panic(fmt.Errorf("ER003: Fatal error - reading config file: %s", err.Error()))
	}
	// Limit the number of visible room regardless of config file
	// (hard-coded limitation for Released application)
	key = fmt.Sprintf("process.%s.visible-room", process)
	roomCount := viper.GetInt(key)
	if roomCount < 0 {
		roomCount = 0
	} else if roomCount > MAX_ROOM {
		roomCount = MAX_ROOM
	}
	rooms = rooms[:roomCount] //prune

	// Validate data
	if len(rooms) == 0 {
		panic(fmt.Errorf("ER004: Fatal config error - missing room details"))
	}

	// Map room. We can't directly marshal to map because we add hard-coded limitation with trimming
	// which is easier done in slice
	collision := 1
	for i := 0; i < len(rooms); i++ {
		cfg.RoomMap[process][rooms[i].Code] = rooms[i]

		if _, exist := cfg.OrderedRoom[process][rooms[i].Order]; !exist {
			cfg.OrderedRoom[process][rooms[i].Order] = rooms[i].Code
		} else {
			cfg.OrderedRoom[process][rooms[i].Order+collision] = rooms[i].Code
			collision++
		}
	}
}

func (cfg *Config) getBranchInfo(branchCode string) (string, int) {
	// Match URL path {branch} with config file
	branch := ""
	i := 0
	for i < len(cfg.Branches) {
		if cfg.Branches[i].Code == branchCode {
			branch = cfg.Branches[i].Name
			break
		}
		i++
	}

	return branch, i
}

func (cfg *Config) validateBranch(branchCode string) bool {
	exp := regexp.MustCompile(`^[a-z]{3}$`)
	if valid := exp.MatchString(branchCode); !valid {
		return false
	}

	for _, branchRef := range cfg.Branches {
		if branchCode == branchRef.Code {
			return true
		}
	}
	return false
}
