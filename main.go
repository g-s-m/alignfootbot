package main

import (
	"github.com/Syfaro/telegram-bot-api"
	"fmt"
	"log"
	"strings"
	"strconv"
	"reflect"
	"github.com/vrischmann/envconfig"
	"alignfootbot/afdb"
)

type Config struct {
	DbHost    string `envconfig:"DB_HOST"`
	DbPort    string `envconfig:"DB_PORT"`
	DbUser    string `envconfig:"DB_USER"`
	DbName    string `envconfig:"DB_NAME"`
	DbPass    string `envconfig:"DB_PASSW"`
	DbSslMode string `envconfig:"DB_SSL_MODE"`
	BotToken  string `envconfig:"BOT_TOKEN"`
}

func getConfig() *Config {
	var conf Config
	if err := envconfig.Init(&conf); err != nil {
		log.Fatalln(err)
	}
	return &conf
}

func startGame(db *afdb.Db, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	log.Println("start game")
	strTemplate := "Привет, собираемся на игру, деньги принимает %s"
	
	db.NewGame()
	reply := fmt.Sprintf(strTemplate, msg.From.String())
	responce := tgbotapi.NewMessage(msg.Chat.ID, reply)
	bot.Send(responce)
}

func countPlayers(db *afdb.Db, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	log.Println("count players")
	players := db.ChatPlayers(msg.Chat.ID)
    text := "Всего сдали: %f р.\nВсего в банке: %f р.\nОтметились %d человек:\n"
	sum := float64(0)
	count := 0
	for _, player := range players {
		text += player.UserName
		if player.Count > 1 {
			text += " +" + strconv.Itoa(player.Count)
		}
		text += "\n"
		sum += player.Money
		count += player.Count
		log.Printf("(%s, %d, %f)", player.UserName, player.Count, player.Money)
	}
	reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf(text, sum, db.HowMuchMoney(msg.Chat.ID), count))
	bot.Send(reply)
}

func addPlayer(db *afdb.Db, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	log.Println("add player")
	players := 1
	fmt.Sscanf(msg.Text, "+%d", &players)

	db.NewPlayer(msg.Chat.ID, int64(msg.From.ID), msg.From.String(), players)
}

func removePlayer(db *afdb.Db, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	log.Println("remove player")
	players := 1
	fmt.Sscanf(msg.Text, "-%d", &players)
	db.DropPlayer(msg.Chat.ID, int64(msg.From.ID), players)
}

func addMoney(db *afdb.Db, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	log.Println("add money")
	var money float64
	fmt.Sscanf(msg.Text, "$%f", &money)
	db.PutMoney(msg.Chat.ID, int64(msg.From.ID), msg.From.String(), money)
}

func setGameCost(db *afdb.Db, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	log.Println("set game cost")
	var money float64
	log.Printf("Text: %s", msg.Text)
	fmt.Sscanf(msg.CommandArguments(), "%f", &money)
	db.SetGameCost(msg.Chat.ID, money)
}

func handleCommands(db *afdb.Db, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	cmds := map[string]func(*afdb.Db, *tgbotapi.BotAPI, *tgbotapi.Message) {
		"/go"    : startGame,
		"/cost"  : setGameCost,
		"/count" : countPlayers,
	}
	log.Printf("Receive message: %s", msg.Text)
	tokens := strings.Fields(msg.Text)
	if cmd, ok := cmds[tokens[0]]; ok {
		cmd(db, bot, msg)
	}
}

func handleText(db *afdb.Db, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) bool {
	actions := map[byte]func(*afdb.Db, *tgbotapi.BotAPI, *tgbotapi.Message) {
		'+' : addPlayer,
		'-' : removePlayer,
		'$' : addMoney,
		'/' : handleCommands,
	}
	if cmd, ok := actions[msg.Text[0]]; ok {
		cmd(db, bot, msg)
		return true
	}
	return false
}

type Service struct {
	db     *afdb.Db
	botApi *tgbotapi.BotAPI
}

func (th *Service) Run() {
	var ucfg tgbotapi.UpdateConfig = tgbotapi.NewUpdate(0)
	ucfg.Timeout = 60
	updates, err := th.botApi.GetUpdatesChan(ucfg)
	if err != nil {
		log.Panic("Can't get updates: %s", err)
	}
	th.db.CreateMoneyTable()
	for {
		select {
		case update := <-updates:
			if update.Message == nil {
				continue
			}

			if reflect.TypeOf(update.Message.Text).Kind() == reflect.String && update.Message.Text != "" {
				handleText(th.db, th.botApi, update.Message)
			}
		}
	}
}

func (th *Service) Close() {
	th.db.Close()
}

func CreateService(conf *Config) Service {
	db, err := afdb.DbConnect(conf.DbHost, conf.DbPort, conf.DbUser, conf.DbPass, conf.DbName, conf.DbSslMode)
	if err != nil {
		log.Panic("Can't connect to database")
	}
	log.Printf("DB connection is established")

	bot, err := tgbotapi.NewBotAPI(conf.BotToken)
	if err != nil {
		db.Close()
		log.Panic("Can't get API")
	}
	return Service {
		db     : db,
		botApi : bot,
	}
}

func main() {
	conf := getConfig()
	service := CreateService(conf)
	defer service.Close()
	service.Run()
}
