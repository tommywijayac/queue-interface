package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

const MAX_ROOM int = 10

type SessionKey struct {
	Auth    []byte
	Encrypt []byte
}

type RoomData struct {
	Name      string   `mapstructure:"name"`
	GroupCode string   `mapstructure:"group-code"`
	Code      []string `mapstructure:"code"`
	Order     int      `mapstructure:"order"`
}

type BranchData struct {
	Name     string `mapstructure:"name"`
	Code     string `mapstructure:"code"`
	ID       string `mapstructure:"id"`
	Password string `mapstructure:"password"`
}

type Config struct {
	IsDev bool

	Branches []BranchData
	Rooms    map[string][]RoomData
	RoomMap  map[string]map[string]*RoomData //process code -> room code

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

	AppConfig.IsDev = viper.GetBool("ISDEV") //default value (if key not exist) is false

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
	cfg.Rooms = make(map[string][]RoomData)
	cfg.RoomMap = make(map[string]map[string]*RoomData)
	cfg.RoomMap["opr"] = make(map[string]*RoomData)
	cfg.RoomMap["pol"] = make(map[string]*RoomData)

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

	// Save to persisted vars
	cfg.Rooms[process] = make([]RoomData, len(rooms))
	copy(cfg.Rooms[process], rooms)

	// cfg.mapRoom(rooms)
	switch process {
	case "opr":
		cfg.mapOprRoom(cfg.Rooms[process])
	case "pol":
		cfg.mapRoom(cfg.Rooms[process])
	}
}

func (cfg *Config) mapOprRoom(rooms []RoomData) {
	// 27/08/2021: Because of KMN specs that the OPR flow all has same group-code,
	// Then we need to use room-code instead to differentiate, and put them as KEY for OPR flow

	process := "opr"
	for i := 0; i < len(rooms); i++ {
		// Map each room code. This will be used for matching
		for _, room_code := range rooms[i].Code {
			// Standardize key: lowercase
			room_code = strings.ToLower(room_code)

			cfg.RoomMap[process][room_code] = &rooms[i]
		}
	}
}

func (cfg *Config) mapRoom(rooms []RoomData) {
	process := "pol"
	for i := 0; i < len(rooms); i++ {
		// Standardize key: lowercase
		group_code := strings.ToLower(rooms[i].GroupCode)

		cfg.RoomMap[process][group_code] = &rooms[i]
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
