package ws

// HOW TO RUN
// go test ws_cli.go ws_op.go ws_contants.go utils.go ws_priv_channel.go ws_priv_channel_Accout_test.go -v

import (
	"fmt"
	"log"
	"testing"
	"time"
)

const (
	TRADE_ACCOUNT = iota
	ISOLATE_ACCOUNT
	CROSS_ACCOUNT
	CROSS_ACCOUNT_B
)

func prework_pri(t int) *WsClient {
	// 模拟环境
	ep := "wss://wsaws.okex.com:8443/ws/v5/private"
	var apikey, passphrase, secretKey string
	// 把账号密码写这里
	switch t {
	case TRADE_ACCOUNT:
		apikey = "fe468418-5e40-433f-8d04-04951286d417"
		passphrase = "M4pw71Id"
		secretKey = "D6D74DF9DD60A25BE2B27CA71D8F814D"
	case ISOLATE_ACCOUNT:
		apikey = "fe468418-5e40-433f-8d04-04951286d417"
		passphrase = "M4pw71Id"
		secretKey = "D6D74DF9DD60A25BE2B27CA71D8F814D"
	case CROSS_ACCOUNT:
		apikey = "fe468418-5e40-433f-8d04-04951286d417"
		passphrase = "M4pw71Id"
		secretKey = "D6D74DF9DD60A25BE2B27CA71D8F814D"
	case CROSS_ACCOUNT_B:
		apikey = "fe468418-5e40-433f-8d04-04951286d417"
		passphrase = "M4pw71Id"
		secretKey = "D6D74DF9DD60A25BE2B27CA71D8F814D"
	}

	r, err := NewWsClient(ep)
	if err != nil {
		log.Fatal(err)
	}

	err = r.Start()
	if err != nil {
		log.Fatal(err)
	}

	var res bool
	start := time.Now()
	res, _, err = r.Login(apikey, secretKey, passphrase)
	if res {
		usedTime := time.Since(start)
		fmt.Println("登录成功！", usedTime.String())
	} else {
		log.Fatal("登录失败！", err)
	}
	fmt.Println(apikey, secretKey, passphrase)
	return r
}

// 账户频道 测试
func TestAccout(t *testing.T) {
	r := prework_pri(CROSS_ACCOUNT)
	var res bool
	var err error

	var args []map[string]string
	arg := make(map[string]string)
	arg["ccy"] = "BTC"
	args = append(args, arg)
	fmt.Println("args: ", args)
	start := time.Now()
	res, _, err = r.PrivAccout(OP_SUBSCRIBE, args)
	if res {
		usedTime := time.Since(start)
		fmt.Println("订阅所有成功！", usedTime.String())
	} else {
		fmt.Println("订阅所有成功！", err)
		t.Fatal("订阅所有成功！", err)
	}

	time.Sleep(100 * time.Second)
	start = time.Now()
	// res, _, err = r.PrivAccout(OP_UNSUBSCRIBE, args)
	// if res {
	// usedTime := time.Since(start)
	// fmt.Println("取消订阅所有成功！", usedTime.String())
	// } else {
	// fmt.Println("取消订阅所有失败！", err)
	// t.Fatal("取消订阅所有失败！", err)
	// }

}
