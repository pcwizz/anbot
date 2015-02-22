package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/quiteawful/go-ircevent"
	"io/ioutil"
	"log"
	"regexp"
	"strings"
	"time"
)

var config Config

var CompiledDirectInteractions map[string]*regexp.Regexp
var CompiledInteractions map[string]*regexp.Regexp

/*init: set up state expected by this file*/
func init() {
	CompiledDirectInteractions = make(map[string]*regexp.Regexp)
	CompiledInteractions = make(map[string]*regexp.Regexp)
}

func main() {
	loadConfig()
	directexp := regexp.MustCompile(config.Nick + ":.*")
	if config.Debug == true {
		fmt.Printf("\nNickexp:\t%s\n", directexp)
	}
	//built in interactions
	CompiledDirectInteractions[licenceexp] = regexp.MustCompile(licenceexp)
	iplayerexp := regexp.MustCompile(`bbc\.co\.uk/iplayer/episode/\w+/(\w|-)+`)
	youtubeexp := regexp.MustCompile(`((youtube\..*/watch\?)|(youtu\.be/))(\w|-|=|&)+`)
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
	//Handel events
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
