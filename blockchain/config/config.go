package config

import (
	"IS/utils"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type (
	Flag struct {
		Flag             string `json:"flag"`
		Required         bool   `json:"required"`
		Description      string `json:"description"`
		Processor        func(*Config, string) error
		DefaultProcessor func(*Config, string)
	}
	BoolFlag struct {
		Flag
		DefaultValue bool `json:"defaultValue"`
	}
	ValueFlag struct {
		Flag
		DefaultValue string `json:"defaultValue"`
	}

	Config struct {
		DataDirectory              string
		HttpAddress                string
		HttpPort                   string
		LogsDir                    string
		DataBaseAddress            string
		DataBasePort               string
		DataBaseName               string
		UsersCollectionName        string
		StateCollectionName        string
		TransactionsCollectionName string
		ApiOnly                    bool // no tg bot
	}
)

func (bf BoolFlag) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Flag         string `json:"flag"`
		Required     bool   `json:"required"`
		Description  string `json:"description"`
		DefaultValue bool   `json:"defaultValue"`
	}{
		Flag:         bf.FlagValue(),
		Required:     bf.Required,
		Description:  bf.Description,
		DefaultValue: bf.DefaultValue,
	})
}

func (bf ValueFlag) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Flag         string `json:"flag"`
		Required     bool   `json:"required"`
		Description  string `json:"description"`
		DefaultValue string `json:"defaultValue"`
	}{
		Flag:         bf.FlagValue(),
		Required:     bf.Required,
		Description:  bf.Description,
		DefaultValue: bf.DefaultValue,
	})
}

func (f *Flag) FlagValue() string {
	return f.Flag
}

var (
	helpFlag = BoolFlag{ // special flag 🤗
		Flag: Flag{
			Flag: "--help",

			Required:    false,
			Description: "show all flags",
		},
		DefaultValue: false,
	}
	boolFlags = []BoolFlag{
		{
			Flag: Flag{
				Flag:        "--disable-tg-bot",
				Required:    false,
				Description: "API only",
				Processor: func(config *Config, s string) error {
					config.ApiOnly = true
					return nil
				},
			},
			DefaultValue: false,
		},
	}
	valueFlags = []ValueFlag{
		{
			Flag: Flag{
				Flag:        "--data-dir",
				Required:    false,
				Description: "change default data directory",
				Processor: func(config *Config, data string) error {
					_, err := os.ReadDir(data)
					if err != nil {
						return fmt.Errorf("unknown directory: \"%v\"", data)
					}
					config.DataDirectory = data
					return nil
				},
				DefaultProcessor: func(config *Config, defaultValue string) {
					config.DataDirectory = defaultValue
				},
			},
			DefaultValue: ".data/node",
		},
		{
			Flag: Flag{
				Flag:        "--http-addr",
				Required:    false,
				Description: "change default address",
				Processor: func(config *Config, data string) error {
					config.HttpPort = data
					return nil
				},
				DefaultProcessor: func(config *Config, defaultValue string) {
					config.HttpAddress = defaultValue
				},
			},
			DefaultValue: "localhost",
		},
		{
			Flag: Flag{
				Flag:        "--http-port",
				Required:    false,
				Description: "set HTTP port",
				Processor: func(config *Config, data string) error {
					if _, err := strconv.Atoi(data); err != nil {
						return fmt.Errorf("invalid port: \"%v\"", data)
					}
					config.HttpPort = data
					return nil
				},
				DefaultProcessor: func(config *Config, defaultValue string) {
					config.HttpPort = defaultValue
				},
			},
			DefaultValue: "9545",
		},
		{
			Flag: Flag{
				Flag:        "--logs-dir",
				Required:    false,
				Description: "set output logs directory",
				Processor: func(config *Config, data string) error {
					_, err := os.ReadDir(data)
					if err != nil {
						return fmt.Errorf("unknown directory: \"%v\"", data)
					}
					config.LogsDir = data
					return nil
				},
			},
		},
		{
			Flag: Flag{
				Flag:        "--db-addr",
				Required:    true,
				Description: "set address to connect mongo",
				Processor: func(config *Config, data string) error {
					config.DataBaseAddress = data
					return nil
				},
			},
		},
		{
			Flag: Flag{
				Flag:        "--db-port",
				Required:    true,
				Description: "set port to connect mongo",
				Processor: func(config *Config, data string) error {
					config.DataBasePort = data
					return nil
				},
			},
		},
		{
			Flag: Flag{
				Flag:        "--db-name",
				Required:    false,
				Description: "set db name to use right database_utils",
				Processor: func(config *Config, data string) error {
					config.DataBaseName = data
					return nil
				},
				DefaultProcessor: func(config *Config, defaultValue string) {
					config.DataBaseName = defaultValue
				},
			},
			DefaultValue: "BC",
		},
		{
			Flag: Flag{
				Flag:        "--users-collection-name",
				Required:    false,
				Description: "set users collection name to use right collection",
				Processor: func(config *Config, data string) error {
					config.UsersCollectionName = data
					return nil
				},
				DefaultProcessor: func(config *Config, defaultValue string) {
					config.UsersCollectionName = defaultValue
				},
			},
			DefaultValue: "users",
		},
		{
			Flag: Flag{
				Flag:        "--state-collection-name",
				Required:    false,
				Description: "set state collection name to use right collection",
				Processor: func(config *Config, data string) error {
					config.StateCollectionName = data
					return nil
				},
				DefaultProcessor: func(config *Config, defaultValue string) {
					config.StateCollectionName = defaultValue
				},
			},
			DefaultValue: "state",
		},
		{
			Flag: Flag{
				Flag:        "--txs-collection-name",
				Required:    false,
				Description: "set txs collection name to use right collection",
				Processor: func(config *Config, data string) error {
					config.TransactionsCollectionName = data
					return nil
				},
				DefaultProcessor: func(config *Config, defaultValue string) {
					config.TransactionsCollectionName = defaultValue
				},
			},
			DefaultValue: "transactions",
		},
	}
	boolFlagsValues     []string
	valueFlagsValues    []string
	allFlagsValues      []string
	requiredFlagsValues []string
)

func GetFlagByValue(value string) interface{} { // TODO rm if unused
	if ok, _ := utils.Contains(value, boolFlagsValues); ok {
		for _, flag := range boolFlags {
			if flag.FlagValue() == value {
				return flag
			}
		}
	}
	if ok, _ := utils.Contains(value, valueFlagsValues); ok {
		for _, flag := range valueFlags {
			if flag.FlagValue() == value {
				return flag
			}
		}
	}
	return nil
}

func ArgsToConfig(args []string) (*Config, error) {
	if ok, _ := utils.Contains(helpFlag.FlagValue(), args); ok {
		for _, flag := range valueFlags {
			JSON, _ := json.Marshal(flag)
			fmt.Println(string(JSON))
		}
		for _, flag := range boolFlags {
			JSON, _ := json.Marshal(flag)
			fmt.Println(string(JSON))
		}
		return nil, fmt.Errorf("")
	}
	for _, flag := range allFlagsValues {
		if _, indexes := utils.Contains(flag, args); len(indexes) > 1 {
			panic("flag redeclaration")
		}
	}
	for _, reqFlag := range requiredFlagsValues {
		if cont, _ := utils.Contains(reqFlag, args); !cont {
			panic(fmt.Sprintf("can't find required flag: %v", reqFlag))
		}
	}
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") && len(arg) == 2 || strings.HasPrefix(arg, "--") {
			if contains, _ := utils.Contains(arg, allFlagsValues); !contains {
				panic(fmt.Sprintf("Unknown flag: %v", arg))
			}
		}
	}

	cfg := &Config{}
	for _, flag := range boolFlags {
		if cont, _ := utils.Contains(flag.FlagValue(), args); cont {
			if flag.Processor != nil {
				err := flag.Processor(cfg, "")
				if err != nil {
					return nil, err
				}
			}
		} else {
			if flag.DefaultProcessor != nil {
				flag.DefaultProcessor(cfg, "")
			}
		}
	}

	for _, flag := range valueFlags {
		if cont, indexes := utils.Contains(flag.FlagValue(), args); cont {
			if len(args) == indexes[0]+1 {
				panic(fmt.Sprintf("missing value for flag %v", flag.FlagValue()))
			}
			if flag.Processor != nil {
				err := flag.Processor(cfg, args[indexes[0]+1])
				if err != nil {
					return nil, err
				}
			}
		} else {
			if flag.DefaultProcessor != nil {
				flag.DefaultProcessor(cfg, flag.DefaultValue)
			}
		}
	}

	return cfg, nil
}

func init() {
	for _, flag := range boolFlags {
		boolFlagsValues = append(boolFlagsValues, flag.FlagValue())
		if flag.Required {
			requiredFlagsValues = append(requiredFlagsValues, flag.FlagValue())
		}
	}
	for _, flag := range valueFlags {
		valueFlagsValues = append(valueFlagsValues, flag.FlagValue())
		if flag.Required {
			requiredFlagsValues = append(requiredFlagsValues, flag.FlagValue())
		}
	}
	allFlagsValues = append(boolFlagsValues, valueFlagsValues...)

	if len(allFlagsValues) != len(utils.Uniq(allFlagsValues)) {
		panic("flags collision")
	}
}
