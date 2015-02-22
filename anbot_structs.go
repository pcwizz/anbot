package main

type timeAlert struct {
	MSG          string `json:msg`
	Hour, Minute int
}

type interaction struct {
	Exp, Resp string
}

type Config struct {
	CurrencyApiKey        string
	Debug                 bool
	TLS                   bool `json:TLS`
	SelfSigned            bool
	Server, Nick, Channel string
	TimeAlerts            []timeAlert
	DirectInteractions    []interaction
	Interactions          []interaction
}

type rate_obj struct {
	Rate   float64
	Expiry int64
} //Relative to GBP; against real money
