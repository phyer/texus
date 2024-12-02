package ws

// HOW TO RUN
// go test ws_cli.go ws_op.go ws_contants.go utils.go ws_pub_channel.go  ws_pub_channel_test.go ws_priv_channel.go  ws_priv_channel_test.go ws_jrpc.go  ws_jrpc_test.go  ws_test_AddBookedDataHook.go -v
import (
	"fmt"
	"log"
	"testing"
	"time"
	. "v5sdk_go/ws/wImpl"
)

const (
	TRADE_ACCOUNT = iota
	ISOLATE_ACCOUNT
	CROSS_ACCOUNT
	CROSS_ACCOUNT_B
)

func init() {
	log.SetFlags(log.LstdFlags | log.Llongfile)

}

func prework() *WsClient {
	ep := "wss://wsaws.okex.com:8443/ws/v5/private"

	r, err := NewWsClient(ep)
	if err != nil {
		log.Fatal(err)
	}

	err = r.Start()
	if err != nil {
		log.Fatal(err, ep)
	}
	return r
}
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

func TestAddBookedDataHook(t *testing.T) {
	var r *WsClient

	/*订阅私有频道*/
	{
		r = prework_pri(CROSS_ACCOUNT)
		var res bool
		var err error

		r.AddBookMsgHook(func(ts time.Time, data MsgData) error {
			// 添加你的方法
			fmt.Println("这是自定义AddBookMsgHook")
			fmt.Println("当前数据是", data)
			return nil
		})

		param := map[string]string{}
		param["channel"] = "account"
		param["ccy"] = "BTC"

		res, _, err = r.Subscribe(param)
		if res {
			fmt.Println("订阅成功！")
		} else {
			fmt.Println("订阅失败！", err)
			t.Fatal("订阅失败！", err)
			//return
		}

		time.Sleep(100 * time.Second)
	}

	//订阅公共频道
	{
		r = prework()
		var res bool
		var err error

		// r.AddBookMsgHook(func(ts time.Time, data MsgData) error {
		// 添加你的方法
		// fmt.Println("这是公共自定义AddBookMsgHook")
		// fmt.Println("当前数据是", data)
		// return nil
		// })

		param := map[string]string{}
		param["channel"] = "instruments"
		param["instType"] = "FUTURES"

		res, _, err = r.Subscribe(param)
		if res {
			fmt.Println("订阅成功！")
		} else {
			fmt.Println("订阅失败！", err)
			t.Fatal("订阅失败！", err)
			//return
		}

		select {}
	}

}
