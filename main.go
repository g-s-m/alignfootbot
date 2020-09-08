package main

import (
	"github.com/Syfaro/telegram-bot-api"
	"database/sql"
	_ "github.com/lib/pq"
	"fmt"
	"log"
	"reflect"
	"github.com/vrischmann/envconfig"
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

func startGame(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	log.Println("start game")
	strTemplate := "Привет, собираемся на игру, деньги принимает %s"

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS footusers (USER_ID SERIAL PRIMARY KEY, TIMESTAMP TIMESTAMP DEFAULT CURRENT_TIMESTAMP, USERNAME TEXT, CHAT_ID INT, AMOUNT INT, MONEY REAL);`); err != nil {
        log.Panic("Can't create table: %s", err)
	}

	reply := fmt.Sprintf(strTemplate, msg.From.String())
	responce := tgbotapi.NewMessage(msg.Chat.ID, reply)
	bot.Send(responce)
}

func countPlayers(*sql.DB, *tgbotapi.BotAPI, *tgbotapi.Message) {
	log.Println("count")
}

func addPlayer(*sql.DB, *tgbotapi.BotAPI, *tgbotapi.Message) {
	log.Println("add player")
}

func removePlayer(*sql.DB, *tgbotapi.BotAPI, *tgbotapi.Message) {
	log.Println("remove player")
}

func addMoney(*sql.DB, *tgbotapi.BotAPI, *tgbotapi.Message) {
	log.Println("add money")
}

func handleCommands(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	cmds := map[string]func(*sql.DB, *tgbotapi.BotAPI, *tgbotapi.Message) {
		"/go"    : startGame,
		"/count" : countPlayers,
	}
	if cmd, ok := cmds[msg.Text]; ok {
		cmd(db, bot, msg)
	}
}

func handleText(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) bool {
	actions := map[byte]func(*sql.DB, *tgbotapi.BotAPI, *tgbotapi.Message) {
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

type Db struct {
	Connection *sql.DB
}

func dbConnect(host string, port string, user string, pswd string, name string, sslMode string) (*Db, error) {
	connection, err := sql.Open("postgres", fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", host, port, user, pswd, name, sslMode))
	return &Db{
		Connection : connection,
	}, err
}

type Service struct {
	db     *Db
	botApi *tgbotapi.BotAPI
}

func (th *Service) Run() {
	var ucfg tgbotapi.UpdateConfig = tgbotapi.NewUpdate(0)
	ucfg.Timeout = 60
	updates, err := th.botApi.GetUpdatesChan(ucfg)
	if err != nil {
		log.Panic("Can't get updates: %s", err)
	}
	for {
		select {
		case update := <-updates:
			if reflect.TypeOf(update.Message.Text).Kind() == reflect.String && update.Message.Text != "" {
				handleText(th.db.Connection, th.botApi, update.Message)
			}
		}
	}
}

func (th *Service) Close() {
	th.db.Connection.Close()
}

func CreateService(conf *Config) Service {
	db, err := dbConnect(conf.DbHost, conf.DbPort, conf.DbUser, conf.DbPass, conf.DbName, conf.DbSslMode)
	if err != nil {
		log.Panic("Can't connect to database")
	}
	log.Printf("DB connection is established")

	bot, err := tgbotapi.NewBotAPI(conf.BotToken)//"1270046039:AAERRjXQV4-o0um6vai_U7e0kJ4WiyQXTWQ")
	if err != nil {
		db.Connection.Close()
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
