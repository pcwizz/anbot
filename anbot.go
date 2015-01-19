package main

import (
	"github.com/quiteawful/go-ircevent"
	"crypto/tls"
	"time"
	"log"
	"encoding/json"
	"io/ioutil"
	"strings"
	"regexp"
)

var config Config

var CompiledDirectInteractions map[string]*regexp.Regexp

const license = "This program is free software: you can redistribute it and/or modify " +
    "it under the terms of the GNU General Public License as published by "+
    "the Free Software Foundation, either version 3 of the License, or "+
    "(at your option) any later version. "+
    "This program is distributed in the hope that it will be useful, "+
    "but WITHOUT ANY WARRANTY; without even the implied warranty of "+
    "MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the "+
    "GNU General Public License for more details. "+
    "You should have received a copy of the GNU General Public License "+
    "along with this program.  If not, see <http://www.gnu.org/licenses/>. "
const licenceexp = `\s*licen[c|s]e\s*`
func main() {
	CompiledDirectInteractions = make(map[string]*regexp.Regexp)
	directexp := regexp.MustCompile(config.Nick + ":.*")
	//built in interactions
	CompiledDirectInteractions[licenceexp] = regexp.MustCompile(licenceexp)
	loadConfig()
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
	irccon.AddCallback("001", func(e *irc.Event) { irccon.Join(config.Channel)})
	irccon.AddCallback("366",
	func (e *irc.Event ) {
		for _, alert := range config.TimeAlerts {
			go TimeAlert(irccon, alert.MSG, alert.Hour, alert.Minute)
		}
	})
	irccon.AddCallback("PRIVMSG",
	func ( e *irc.Event ){
		if directexp.MatchString(e.Arguments[1]) {
			DirectInteraction(irccon, e)
			return
		}
	})
	irccon.Loop()
}

type timeAlert struct{
	MSG string `json:msg`
	Hour, Minute int
}

type interaction struct {
	Exp, Resp string
}

type Config struct{
	Debug bool
	TLS bool `json:TLS`
	SelfSigned bool
	Server, Nick, Channel string
	TimeAlerts []timeAlert
	DirectInteractions []interaction
}

func loadConfig(){
	File, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatal(err)
		return
	}
	if err = json.Unmarshal(File, &config);  err != nil {
		log.Fatal(err)
	}
}

func CompileInteractions(){
	for _, interaction := range config.DirectInteractions{
		CompiledDirectInteractions[interaction.Exp] = regexp.MustCompile(interaction.Exp)
	}
}

func TimeAlert(irccon *irc.Connection, msg string, hour, minute int) {
	seconds := time.Now().Second()
	seconds = 60 - seconds
	time.Sleep(time.Duration(seconds) * time.Second)
	for true {
		if time.Now().Hour() == hour && time.Now().Minute() == minute {
			irccon.Privmsg(config.Channel, msg)
			time.Sleep(12 * time.Hour )
		} else {
			time.Sleep( 1 * time.Minute )
		}
	}
}

func DirectInteraction (irccon *irc.Connection, e *irc.Event){
	parts := strings.SplitAfterN(e.Arguments[1], ":", 2)
	if CompiledDirectInteractions[licenceexp].MatchString(parts[1]) {
		irccon.Privmsg(e.Arguments[0], license)
		return
	}

	for _, interaction := range config.DirectInteractions {
		if CompiledDirectInteractions[interaction.Exp].MatchString(parts[1]){
			irccon.Privmsg(e.Arguments[0], interaction.Resp)
		return
		}
	}
}