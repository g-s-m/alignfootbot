package main

import (
	"alignfootbot/afdb"
	"fmt"
	"github.com/Syfaro/telegram-bot-api"
	"github.com/vrischmann/envconfig"
	"log"
	"reflect"
	"strconv"
	"strings"
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
	if db.GameExists(msg.Chat.ID) {
		gameInfo := db.GameInfo(msg.Chat.ID)
		reply := fmt.Sprintf("@%s уже начал всех собирать: %s", gameInfo.Holder, gameInfo.Comment)
		responce := tgbotapi.NewMessage(msg.Chat.ID, reply)
		bot.Send(responce)
		return
	}
	strTemplate := `Всем привет, собираемся играть, деньги принимает @%s (%s)
Чтобы записаться ставьте "+", если сдали деньги ставьте $200 (значит сдали 200р). Если хотите привести друга, ставьте +2, если передумали, ставьте "-", но деньги не вернем.`
	comment := strings.TrimPrefix(strings.TrimPrefix(msg.Text, "/go"), "@alignfootbot")
	db.NewGame(msg.Chat.ID, msg.From.String(), comment)
	reply := fmt.Sprintf(strTemplate, msg.From.String(), comment)
	responce := tgbotapi.NewMessage(msg.Chat.ID, reply)
	bot.Send(responce)
}

func countPlayers(db *afdb.Db, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	log.Println("count players")
	if !db.GameExists(msg.Chat.ID) {
		reply := fmt.Sprintf("Всего в банке: %f р.", db.HowMuchMoney(msg.Chat.ID))
		responce := tgbotapi.NewMessage(msg.Chat.ID, reply)
		bot.Send(responce)
		return
	}

	players := db.ChatPlayers(msg.Chat.ID)
	text := "Всего сдали: %f р.\nВсего в банке: %f р.\nОтметились %d человек:\n"
	sum := float64(0)
	count := 0
	for _, player := range players {
		text += "@" + player.UserName
		if player.Count > 1 {
			text += " +" + strconv.Itoa(player.Count-1)
		}
		text += "\n"
		sum += player.Money
		count += player.Count
	}
	reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf(text, sum, db.HowMuchMoney(msg.Chat.ID), count))
	bot.Send(reply)
}

func addPlayer(db *afdb.Db, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	log.Println("add player")
	players := 1
	fmt.Sscanf(msg.Text, "+%d", &players)

	text := "записал"
	if !db.NewPlayer(msg.Chat.ID, int64(msg.From.ID), msg.From.String(), players) {
		text = "пока никто не собирался играть, запишись попозже"
	}
	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.BaseChat.ReplyToMessageID = msg.MessageID
	bot.Send(reply)
}

func removePlayer(db *afdb.Db, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	log.Println("remove player")
	players := 1
	fmt.Sscanf(msg.Text, "-%d", &players)
	db.DropPlayer(msg.Chat.ID, int64(msg.From.ID), players)
	text := fmt.Sprintf("ну ладно, в следующий раз приходи")
	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.BaseChat.ReplyToMessageID = msg.MessageID
	bot.Send(reply)
}

func addMoney(db *afdb.Db, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	log.Println("add money")
	var money float64
	fmt.Sscanf(msg.Text, "$%f", &money)
	text := fmt.Sprintf("принял")
	if !db.PutMoney(msg.Chat.ID, int64(msg.From.ID), msg.From.String(), money) {
		text = "пока никто не собирался играть"
	}
	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.BaseChat.ReplyToMessageID = msg.MessageID
	bot.Send(reply)
}

func setGameCost(db *afdb.Db, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	log.Println("set game cost")
	var money float64
	log.Printf("Text: %s", msg.Text)
	fmt.Sscanf(msg.CommandArguments(), "%f", &money)
	db.SetGameCost(msg.Chat.ID, money)
	text := fmt.Sprintf("запомнил")
	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	bot.Send(reply)
}

func finishGame(db *afdb.Db, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	log.Println("finish game")
	if !db.GameExists(msg.Chat.ID) {
		reply := "никто и не собирался"
		responce := tgbotapi.NewMessage(msg.Chat.ID, reply)
		bot.Send(responce)
		return
	}
	playersList := ""
	players := db.ChatPlayers(msg.Chat.ID)
	for _, player := range players {
		playersList += "@" + player.UserName
		if player.Count > 1 {
			playersList += " +" + strconv.Itoa(player.Count-1)
		}
		playersList += ", "
	}

	db.PayForTheGame(msg.Chat.ID)
	text := fmt.Sprintf("%s cпасибо за игру, в банке осталось %f", playersList, db.HowMuchMoney(msg.Chat.ID))
	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	bot.Send(reply)
}

func handleCommands(db *afdb.Db, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	cmds := map[string]func(*afdb.Db, *tgbotapi.BotAPI, *tgbotapi.Message){
		"/go":                  startGame,
		"/cost":                setGameCost,
		"/count":               countPlayers,
		"/finish":              finishGame,
		"/go@alignfootbot":     startGame,
		"/cost@alignfootbot":   setGameCost,
		"/count@alignfootbot":  countPlayers,
		"/finish@alignfootbot": finishGame,
	}
	tokens := strings.Fields(msg.Text)
	if cmd, ok := cmds[tokens[0]]; ok {
		cmd(db, bot, msg)
	}
}

func handleText(db *afdb.Db, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) bool {
	actions := map[byte]func(*afdb.Db, *tgbotapi.BotAPI, *tgbotapi.Message){
		'+': addPlayer,
		'-': removePlayer,
		'$': addMoney,
		'/': handleCommands,
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
	th.db.Init()
	for update := range updates {
		if update.Message == nil {
			continue
		}
		if reflect.TypeOf(update.Message.Text).Kind() == reflect.String && update.Message.Text != "" {
			handleText(th.db, th.botApi, update.Message)
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
	return Service{
		db:     db,
		botApi: bot,
	}
}

func main() {
	conf := getConfig()
	service := CreateService(conf)
	defer service.Close()
	service.Run()
}
