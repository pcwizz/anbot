package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/quiteawful/go-ircevent"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var config Config

var CompiledDirectInteractions map[string]*regexp.Regexp
var CompiledInteractions map[string]*regexp.Regexp

const license = "This program is free software: you can redistribute it and/or modify " +
	"it under the terms of the GNU General Public License as published by " +
	"the Free Software Foundation, either version 3 of the License, or " +
	"(at your option) any later version. " +
	"This program is distributed in the hope that it will be useful, " +
	"but WITHOUT ANY WARRANTY; without even the implied warranty of " +
	"MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the " +
	"GNU General Public License for more details. " +
	"You should have received a copy of the GNU General Public License " +
	"along with this program.  If not, see <http://www.gnu.org/licenses/>. "

const licenceexp = `\s*licen[c|s]e\s*`

type rate_obj struct {
	Rate   float64
	Expiry int64
} //Relative to GBP; against real money
var CurrencyRates map[string]rate_obj

const currencyexp = `(\$|£|€|(Fr\.)|(SFr\.)|(FS)|(BTC)) ?(((\d{1,3}[, ])(\d{3}[, ])*\d{3})|\d+)( ?[.,] ?(\d{1,2}))?`
const floatexp = `^(\+|-)?(((\d{1,3}[, ])(\d{3}[ ,])*\d{3})|\d+)( ?([\.,]) ?(\d{3}[, ])*\d+)?$`

var CompiledRegex map[string]*regexp.Regexp

func main() {
	loadConfig()
	CompiledDirectInteractions = make(map[string]*regexp.Regexp)
	CompiledInteractions = make(map[string]*regexp.Regexp)
	CompiledRegex = make(map[string]*regexp.Regexp)
	CurrencyRates = make(map[string]rate_obj)
	directexp := regexp.MustCompile(config.Nick + ":.*")
	if config.Debug == true {
		fmt.Printf("\nNickexp:\t%s\n", directexp)
	}
	//built in interactions
	CompiledDirectInteractions[licenceexp] = regexp.MustCompile(licenceexp)
	iplayerexp := regexp.MustCompile(`bbc\.co\.uk/iplayer/episode/\w+/(\w|-)+`)
	youtubeexp := regexp.MustCompile(`((youtube\..*/watch\?)|(youtu\.be/))(\w|-|=|&)+`)
	CompiledRegex[currencyexp] = regexp.MustCompile(currencyexp)
	CompiledRegex[floatexp] = regexp.MustCompile(floatexp)
	CompileInteractions()
	irccon := irc.IRC(config.Nick, config.Nick)
	irccon.Debug = config.Debug
	irccon.VerboseCallbackHandler = true
	irccon.UseTLS = config.TLS
	irccon.TLSConfig = &tls.Config{InsecureSkipVerify: config.SelfSigned}
	err := irccon.Connect(config.Server)
	if err != nil {
		log.Fatal(err)
	}
	irccon.AddCallback("001", func(e *irc.Event) { irccon.Join(config.Channel) })
	irccon.AddCallback("366",
		func(e *irc.Event) {
			for _, alert := range config.TimeAlerts {
				go TimeAlert(irccon, alert.MSG, alert.Hour, alert.Minute)
			}
		})
	irccon.AddCallback("PRIVMSG",
		func(e *irc.Event) {
			if directexp.MatchString(e.Arguments[1]) {
				DirectInteraction(irccon, e)
				return
			}
			if episodeurl := iplayerexp.FindString(e.Arguments[1]); episodeurl != "" {
				get_iplayerCmdGenerator(irccon, episodeurl)
				return
			}
			if youtubeurl := youtubeexp.FindString(e.Arguments[1]); youtubeurl != "" {
				youtube_dlCmdGenerator(irccon, youtubeurl)
				return
			}
			for _, value_str := range CompiledRegex[currencyexp].FindAllString(e.Arguments[1], -1) {
				if value_str != "" {
					if e.Arguments[0] == config.Channel {
						CurrencyExchangeHandler(irccon, value_str, config.Channel)
					} else {
						CurrencyExchangeHandler(irccon, value_str, e.Nick)
					}
				}
			}
			Interaction(irccon, e)
		})
	irccon.Loop()
}

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

func loadConfig() {
	File, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatal(err)
		return
	}
	if err = json.Unmarshal(File, &config); err != nil {
		log.Fatal(err)
	}
}

func CompileInteractions() {
	for _, interaction := range config.DirectInteractions {
		CompiledDirectInteractions[interaction.Exp] = regexp.MustCompile(interaction.Exp)
	}
	for _, interaction := range config.Interactions {
		CompiledInteractions[interaction.Exp] = regexp.MustCompile(interaction.Exp)
	}
}

func TimeAlert(irccon *irc.Connection, msg string, hour, minute int) {
	seconds := time.Now().Second()
	seconds = 60 - seconds
	time.Sleep(time.Duration(seconds) * time.Second)
	for true {
		if time.Now().Hour() == hour && time.Now().Minute() == minute {
			irccon.Privmsg(config.Channel, msg)
			time.Sleep(24 * time.Hour)
		} else {
			var hours, minutes int
			hours = (hour - time.Now().Hour()) % 24
			minutes = (minutes - time.Now().Minute()) % 60
			time.Sleep((time.Duration(minutes) * time.Minute) + (time.Duration(hours) * time.Hour))
		}
	}
}

func DirectInteraction(irccon *irc.Connection, e *irc.Event) {
	parts := strings.SplitAfterN(e.Arguments[1], ":", 2)
	if CompiledDirectInteractions[licenceexp].MatchString(parts[1]) {
		irccon.Privmsg(e.Arguments[0], license)
		return
	}

	for _, interaction := range config.DirectInteractions {
		if CompiledDirectInteractions[interaction.Exp].MatchString(parts[1]) {
			irccon.Privmsg(e.Arguments[0], interaction.Resp)
			return
		}
	}
}

func Interaction(irccon *irc.Connection, e *irc.Event) {
	for _, interaction := range config.Interactions {
		if CompiledInteractions[interaction.Exp].MatchString(e.Arguments[1]) {
			irccon.Privmsg(e.Arguments[0], interaction.Resp)
			return
		}
	}
}

func get_iplayerCmdGenerator(irccon *irc.Connection, url string) {
	irccon.Privmsg(config.Channel, "get_iplayer --tvmode=best --stream "+url+" | mpv -")
}

func youtube_dlCmdGenerator(irccon *irc.Connection, url string) {
	irccon.Privmsg(config.Channel, "youtube-dl -f webm '"+url+"'")
}

func updateExchangeRate(code string) error {
	var rate float64
	switch code {
	case "USD", "EUR", "CHF":
		resp, err := http.Get("http://currency-api.appspot.com/api/GBP/" + code + ".json?key=" + config.CurrencyApiKey)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		var data struct {
			Success bool
			Source  string
			Target  string
			Rate    float64
			Message string
		}
		err = json.Unmarshal(body, &data)
		if err != nil {
			return err
		}
		if data.Success != true {
			err = errors.New(data.Message)
			return err
		}
		rate = data.Rate
		break
	case "BTC":
		resp, err := http.Get("https://bitpay.com/api/rates/GBP")
		if err != nil {
			return err
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		var data struct {
			Code string
			Name string
			Rate float64
		}
		err = json.Unmarshal(body, &data)
		if err != nil {
			return err
		}
		rate = 1 / data.Rate //Bit pay gives us GBP/BTC we want BTC/GBP
		break
	default:
		err := errors.New("Unsupported currency code")
		return err
	}
	CurrencyRates[code] = rate_obj{
		Rate:   rate,
		Expiry: time.Now().Unix() + (12 * 60 * 60), //12 hour expiry
	}
	return nil
}

func GetExchangeRate(code string) (float64, error) {
	if CurrencyRates[code].Rate == 0 || CurrencyRates[code].Expiry <= time.Now().Unix() {
		err := updateExchangeRate(code)
		return CurrencyRates[code].Rate, err
	}
	return CurrencyRates[code].Rate, nil
}

func ConvertCurrency(source, target string, value float64) (amount float64, err error) {
	if source == target {
		amount = value
		return
	}
	var src_rate, tgt_rate float64
	if source != "GBP" {
		src_rate, err = GetExchangeRate(source)
		if err != nil {
			return
		}
	} else {
		src_rate = 1.0
	}
	if target != "GBP" {
		tgt_rate, err = GetExchangeRate(target)
		if err != nil {
			return
		}
	} else {
		tgt_rate = 1.0
	}
	amount = (value * (1 / src_rate)) * tgt_rate
	return
}

func str_to_flt(s string) (f float64, err error) {
	if !CompiledRegex[floatexp].MatchString(s) {
		err = errors.New("str_to_flt: Input not a float")
	}
	//Get sign
	sign := false //True if negative
	if strings.HasPrefix(s, "-") {
		sign = true
		s = strings.TrimPrefix(s, "-")
	} else if strings.HasPrefix(s, "+") {
		s = strings.TrimPrefix(s, "+")
	}

	// Normalise s so the decimal point is always a .
	p := CompiledRegex[floatexp].FindStringSubmatchIndex(s)[15]
	if p != -1 { //Check there is a [.,]
		if string(s[p-1]) == "," {
			s = s[:p-1] + "." + s[p:]
		}
	}
	var pos, point int //Number of digit positions away from the left most digit left
	/*Point is the number of digit positions away from the left most digit to the digit immediately left of the decimal point*/
	digits := make(map[int]int) //Using a map like an array because I'm number of digits in a string
	for s != "" {
		switch {
		case strings.HasPrefix(s, "0"):
			digits[pos] = 0
			pos++

		case strings.HasPrefix(s, "1"):
			digits[pos] = 1
			pos++

		case strings.HasPrefix(s, "2"):
			digits[pos] = 2
			pos++

		case strings.HasPrefix(s, "3"):
			digits[pos] = 3
			pos++

		case strings.HasPrefix(s, "4"):
			digits[pos] = 4
			pos++

		case strings.HasPrefix(s, "5"):
			digits[pos] = 5
			pos++

		case strings.HasPrefix(s, "6"):
			digits[pos] = 6
			pos++

		case strings.HasPrefix(s, "7"):
			digits[pos] = 7
			pos++

		case strings.HasPrefix(s, "8"):
			digits[pos] = 8
			pos++

		case strings.HasPrefix(s, "9"):
			digits[pos] = 9
			pos++

		case strings.HasPrefix(s, "."):
			point = pos
		}
		s = s[1:] //Remove first rune from string
	}
	//Set the point to after last digit if we didn't set it
	if point == 0 {
		point = len(digits)
	}
	for pos, digit := range digits {
		exp := -pos + point - 1
		f += float64(digit) * math.Pow10(exp)
	}
	if sign {
		f = -f
	}
	return
}

func CurrencyExchangeHandler(irccon *irc.Connection, value, nick string) {
	var code string
	//Work out currency
	switch {
	case strings.HasPrefix(value, "$"):
		code = "USD"
		value = strings.TrimPrefix(value, "$")
	case strings.HasPrefix(value, "£"):
		code = "GBP"
		value = strings.TrimPrefix(value, "£")
	case strings.HasPrefix(value, "€"):
		code = "EUR"
		value = strings.TrimPrefix(value, "€")
	case strings.HasPrefix(value, "Fr."):
		code = "CHF"
		value = strings.TrimPrefix(value, "Fr.")
	case strings.HasPrefix(value, "SFr."):
		code = "CHF"
		value = strings.TrimPrefix(value, "SFr.")
	case strings.HasPrefix(value, "FS"):
		code = "CHF"
		value = strings.TrimPrefix(value, "FS")
	case strings.HasPrefix(value, "BTC"):
		code = "BTC"
		value = strings.TrimPrefix(value, "BTC")
	}
	if code == "" {
		log.Fatal(errors.New("Currency not recognised"))
	}
	value = strings.Trim(value, " ") //trim white space
	value_flt, err := str_to_flt(value)
	if err != nil {
		log.Fatal(err)
	}
	USD, err := ConvertCurrency(code, "USD", value_flt)
	if err != nil {
		log.Fatal(err)
	}
	GBP, err := ConvertCurrency(code, "GBP", value_flt)
	if err != nil {
		log.Fatal(err)
	}
	EUR, err := ConvertCurrency(code, "EUR", value_flt)
	if err != nil {
		log.Fatal(err)
	}
	CHF, err := ConvertCurrency(code, "CHF", value_flt)
	if err != nil {
		log.Fatal(err)
	}
	BTC, err := ConvertCurrency(code, "BTC", value_flt)
	if err != nil {
		log.Fatal(err)
	}
	s := fmt.Sprintf("$ %.2f\t|\t£ %.2f\t|\t€ %.2f\t|\tFS %.2f\t|\tBTC %E", USD, GBP, EUR, CHF, BTC)
	irccon.Privmsg(nick, s)
}
