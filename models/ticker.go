package models

type Ticker struct {
	InstType  string  `json:"instType,omitempty"`  //SWAP,
	InstId    string  `json:"instId,omitempty"`    //LTC-USD-SWAP,
	Last      float64 `json:"last,omitempty"`      //9999.99,
	LastSz    float64 `json:"lastSz,omitempty"`    //0.1,
	AskPx     float64 `json:"askPx,omitempty"`     //9999.99,
	AskSz     int32   `json:"askSz,omitempty"`     //11,
	bidPx     float64 `json:"bidPx,omitempty"`     //8888.88,
	BidSz     int32   `json:"bidSz,omitempty"`     //5,
	Open24h   int32   `json:"open24h,omitempty"`   //9000,
	High24h   int32   `json:"high24h,omitempty"`   //10000,
	Low24h    float64 `json:"low24h,omitempty"`    //8888.88,
	VolCcy24h int32   `json:"volCcy24h,omitempty"` //2222,
	Vol24h    int32   `json:"vol24h,omitempty"`    //2222,
	SodUtc0   int32   `json:"sodUtc0,omitempty"`   //2222,
	SodUtc8   int32   `json:"sodUtc8,omitempty"`   //2222,
	Ts        int64   `json:"ts,omitempty"`        //1597026383085
}
