package models

import (
	"phyer.click/tunas/utils"
)

type GlobalCoin struct {
	CoinName      string                                `json:"coinName"`
	Instrument    *Instrument                           `json:"instrument"`
	Ticker        *Ticker                               `json:"ticker"`
	CandleMapList map[string](map[string]utils.MyStack) `json:"candleMapList"` // map["BTC"]["oneMintue"]
}

func (coin *GlobalCoin) GetInstrument() *Instrument {
	return coin.Instrument
}
func (coin *GlobalCoin) SetInstrument(instr *Instrument) {
	coin.Instrument = instr
}

// gcl := map[string]GlobalGoin{}
