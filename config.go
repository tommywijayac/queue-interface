package main

import (
	"fmt"
	"regexp"

	"github.com/spf13/viper"
)

const MAX_ROOM int = 10

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
		ErrorLogger.Fatalf("fail to open config.env. %v\n", err)
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
		ErrorLogger.Fatalf("fail to open config.json. %v\n", err)
	}

	// Read branch configuration
	err = viper.UnmarshalKey("branch", &cfg.Branches)
	if err != nil {
		ErrorLogger.Fatalf("fail to load branch info from config. %v\n", err)
	}
	if len(cfg.Branches) == 0 {
		ErrorLogger.Fatalln("no branch endpoint defined in config (possible corrupted file).")
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
		InfoLogger.Printf("%v is set with default value.\n", key)
	}
}

func readEnvStringConfig(key string, dest *string, default_value string) {
	if temp := viper.GetString(key); temp != "" {
		*dest = temp
	} else {
		*dest = default_value
		InfoLogger.Printf("%v is set with default value.\n", key)
	}
}

// Helper function to simplify room config assignment for each process
func (cfg *Config) readRoomConfig(process string) {
	var rooms []RoomData
	var key string

	key = fmt.Sprintf("process.%s.room", process)
	err := viper.UnmarshalKey(key, &rooms)
	if err != nil {
		ErrorLogger.Fatalf("fail to load room info from config. %v\n", err)
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
		ErrorLogger.Fatalln("missing room list defined in config (possible corrupted or excessive prune).")
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

func (cfg *Config) getBranchInfo(branchCode string) (string, string) {
	branchName, branchID := "", ""
	i := 0
	for i < len(cfg.Branches) {
		if cfg.Branches[i].Code == branchCode {
			branchName = cfg.Branches[i].Name
			branchID = cfg.Branches[i].ID
			break
		}
		i++
	}

	return branchName, branchID
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
