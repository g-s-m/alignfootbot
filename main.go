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
	
	if _, err := db.Exec(`DROP TABLE IF EXISTS game`); err != nil {
		log.Panic("Can't drop previous game")
	}
	if _, err := db.Exec(`CREATE TABLE game (USER_ID INT PRIMARY KEY, USERNAME TEXT, CHAT_ID INT, PLAYERS INT, MONEY REAL);`); err != nil {
        log.Panic("Can't create table: %s", err)
	}

	reply := fmt.Sprintf(strTemplate, msg.From.String())
	responce := tgbotapi.NewMessage(msg.Chat.ID, reply)
	bot.Send(responce)
}

func countPlayers(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	log.Println("count players")
	data := fmt.Sprintf("SELECT (PLAYERS, MONEY, USERNAME) FROM game WHERE CHAT_ID = %d;", msg.Chat.ID)
	rows, err := db.Query(data)
	defer rows.Close()

	if err != nil {
		log.Printf("Error. Query error: %s", err)
	}

	for rows.Next() {
		var (
			result     string
			userName   string
			players    int64
			money      float64
		)
		if err := rows.Scan(&result); err != nil {
			log.Fatal(err)
		}
		fmt.Sscanf(result[0:len(result)-1], "(%d,%f,%s)", &players, &money, &userName)
		log.Printf("(%s, %d, %f)\n", userName, players, money)
	}
}

func addPlayer(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	log.Println("add player")
	players := 1
	fmt.Sscanf(msg.Text, "+%d", &players)
	data := `INSERT INTO game (USER_ID, USERNAME, CHAT_ID, PLAYERS, MONEY) VALUES($1, $2, $3, $4, $5) ON CONFLICT (USER_ID) DO UPDATE SET PLAYERS=game.PLAYERS+$4;`
	result, err := db.Exec(data, msg.From.ID, msg.From.String(), msg.Chat.ID, players, 0)
	if err != nil {
		log.Printf("Error. Can't add player: %s", err)
	}
	ra, err := result.RowsAffected()
	log.Printf("Updated(inserted) %d rows", ra)
}

func removePlayer(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	log.Println("remove player")
	players := 1
	fmt.Sscanf(msg.Text, "-%d", &players)
	data := `UPDATE game SET PLAYERS=game.PLAYERS-$1;`
	result, err := db.Exec(data, players)
	if err != nil {
		log.Printf("Error. Can't remove player: %s", err)
	}
	ra, err := result.RowsAffected()
	log.Printf("Updated(inserted) %d rows", ra)

	data = `DELETE FROM game where PLAYERS <= 0;`
	result, err = db.Exec(data)
	if err != nil {
		log.Printf("Error. Can't remove row: %s", err)
	}
}

func addMoney(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	log.Println("add money")
	var money float64
	fmt.Sscanf(msg.Text, "$%f", &money)
	log.Printf("Insert(update) money=%f", money)
	data := `INSERT INTO game (USER_ID, USERNAME, CHAT_ID, PLAYERS, MONEY) VALUES($1, $2, $3, 1, $4) ON CONFLICT (USER_ID) DO UPDATE SET MONEY=$4;`
	result, err := db.Exec(data, msg.From.ID, msg.From.String(), msg.Chat.ID, money)
	if err != nil {
		log.Printf("Error. Can't add player: %s", err)
	}
	ra, err := result.RowsAffected()
	log.Printf("Updated(inserted) %d rows", ra)

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

	bot, err := tgbotapi.NewBotAPI(conf.BotToken)
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
