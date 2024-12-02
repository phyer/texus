package core

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	simple "github.com/bitly/go-simplejson"
)

type MyConfig struct {
	Env                    string `json:"env", string`
	Config                 *simple.Json
	CandleDimentions       []string          `json:"candleDimentions"`
	RedisConf              *RedisConfig      `json:"redis"`
	CredentialReadOnlyConf *CredentialConfig `json:"credential"`
	CredentialMutableConf  *CredentialConfig `json:"credential"`
	ConnectConf            *ConnectConfig    `json:"connect"`
	// ThreadsConf    *ThreadsConfig    `json:"threads"`
}

type RedisConfig struct {
	Url      string `json:"url", string`
	Password string `json:"password", string`
	Index    int    `json:"index", string`
}

type CredentialConfig struct {
	SecretKey          string `json:"secretKey", string`
	BaseUrl            string `json:"baseUrl", string`
	OkAccessKey        string `json:"okAccessKey", string`
	OkAccessPassphrase string `json:"okAccessPassphrase", string`
}

type ConnectConfig struct {
	LoginSubUrl      string `json:"loginSubUrl", string`
	WsPrivateBaseUrl string `json:"wsPrivateBaseUrl", string`
	WsPublicBaseUrl  string `json:"wsPublicBaseUrl", string`
	RestBaseUrl      string `json:"restBaseUrl", string`
}
type ThreadsConfig struct {
	MaxLenTickerStream int `json:"maxLenTickerStream", int`
	MaxCandles         int `json:"maxCandles", string`
	AsyncChannels      int `json:"asyncChannels", int`
	MaxTickers         int `json:"maxTickers", int`
	RestPeriod         int `json:"restPeriod", int`
	WaitWs             int `json:"waitWs", int`
}

func (cfg MyConfig) Init() (MyConfig, error) {
	env := os.Getenv("GO_ENV")
	arystr := os.Getenv("TUNAS_CANDLESDIMENTIONS")
	ary := strings.Split(arystr, "|")
	cfg.CandleDimentions = ary
	jsonStr, err := ioutil.ReadFile("/go/json/basicConfig.json")
	if err != nil {
		jsonStr, err = ioutil.ReadFile("configs/basicConfig.json")
		if err != nil {
			fmt.Println("err2:", err.Error())
			return cfg, err
		}
		cfg.Config, err = simple.NewJson([]byte(jsonStr))
		if err != nil {
			fmt.Println("err2:", err.Error())
			return cfg, err
		}
		cfg.Env = env
	}
	cfg.Config = cfg.Config.Get(env)

	ru, err := cfg.Config.Get("redis").Get("url").String()
	rp, _ := cfg.Config.Get("redis").Get("password").String()
	ri, _ := cfg.Config.Get("redis").Get("index").Int()
	redisConf := RedisConfig{
		Url:      ru,
		Password: rp,
		Index:    ri,
	}
	// fmt.Println("cfg: ", cfg)
	cfg.RedisConf = &redisConf
	ls, _ := cfg.Config.Get("connect").Get("loginSubUrl").String()
	wsPub, _ := cfg.Config.Get("connect").Get("wsPrivateBaseUrl").String()
	wsPri, _ := cfg.Config.Get("connect").Get("wsPublicBaseUrl").String()
	restBu, _ := cfg.Config.Get("connect").Get("restBaseUrl").String()
	connectConfig := ConnectConfig{
		LoginSubUrl:      ls,
		WsPublicBaseUrl:  wsPub,
		WsPrivateBaseUrl: wsPri,
		RestBaseUrl:      restBu,
	}
	cfg.ConnectConf = &connectConfig
	return cfg, nil
}

func (cfg *MyConfig) GetConfigJson(arr []string) *simple.Json {
	env := os.Getenv("GO_ENV")
	fmt.Println("env: ", env)
	cfg.Env = env

	json, err := ioutil.ReadFile("/go/json/basicConfig.json")

	if err != nil {
		json, err = ioutil.ReadFile("configs/basicConfig.json")
		if err != nil {
			log.Panic("read config error: ", err.Error())
		}
	}
	if err != nil {
		fmt.Println("read file err: ", err)
	}
	rjson, err := simple.NewJson(json)
	if err != nil {
		fmt.Println("newJson err: ", err)
	}
	for _, s := range arr {
		rjson = rjson.Get(s)
		// fmt.Println(s, ": ", rjson)
	}
	return rjson
}
