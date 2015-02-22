package main

import (
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

var CurrencyRates map[string]rate_obj
var CompiledRegex map[string]*regexp.Regexp

func init() {
	CompiledRegex = make(map[string]*regexp.Regexp)
	CurrencyRates = make(map[string]rate_obj)
	CompiledRegex[currencyexp] = regexp.MustCompile(currencyexp)
	CompiledRegex[floatexp] = regexp.MustCompile(floatexp)
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
