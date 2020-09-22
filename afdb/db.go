package afdb

import (
	"database/sql"
	_ "github.com/lib/pq"
	"log"
	"fmt"
	"strings"
	"strconv"
)
type Db struct {
	Connection *sql.DB
}

type Player struct {
	UserName string
	UserId   int64
	Count    int
	Money    float64
}

type Game struct {
	Holder   string
	HolderId int64
	Comment  string
}

func (th *Db) Close() {
	th.Connection.Close()
}

func (th *Db) NewGame(chatId int64, gameHolder string, gameHolderId int64, comment string) {
	log.Printf("Creating tables for chat %d, %s, %s", chatId, gameHolder, comment)
	if _, err := th.Connection.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS game_%d;`, uint64(chatId))); err != nil {
		log.Println("Can't drop previous game")
	}
	if _, err := th.Connection.Exec(fmt.Sprintf(`CREATE TABLE game_%d (USER_ID INT PRIMARY KEY, USERNAME TEXT, PLAYERS INT, MONEY REAL);`, uint64(chatId))); err != nil {
        log.Panic("Can't create table: %s", err)
	}
	if _, err := th.Connection.Exec(`insert into active_games (chat_id, game_holder, game_holder_id, holder_message) 
	                                 values ($1, $2, $3, $4);`, chatId, gameHolder, gameHolderId, comment); err != nil {
		log.Println("Can't insert active game: %s", err)
	}
}

func (th *Db) GameInfo(chatId int64) Game {
	data := fmt.Sprintf("select (game_holder, game_holder_id, holder_message) from active_games where chat_id = %d;", chatId)
	rows, err := th.Connection.Query(data)
	if err != nil {
		log.Printf("Error. Query error: %s", err)
		return Game{}
	}
	defer rows.Close()
	var (
		result   string
		userName string
		userId   int64
		message  string
	)
	if rows.Next() {
		if err := rows.Scan(&result); err != nil {
			log.Fatal(err)
		}
		log.Printf(result)
		result = result[1:len(result) - 1]
		tokens := strings.FieldsFunc(result,
			func(c rune) bool {
				return c == ','
			})
		userName = strings.TrimSuffix(strings.TrimPrefix(tokens[0], `"`), `"`)
		userId, _ = strconv.ParseInt(tokens[1], 10, 64)
		message = strings.TrimSuffix(strings.TrimPrefix(tokens[2], `"`), `"`)
	}
	return Game {
		Holder :   userName,
		HolderId : userId,
		Comment :  message,
	}
}

func (th *Db) ChatPlayers(chatId int64) []Player {
	data := fmt.Sprintf("SELECT (user_id, PLAYERS, MONEY, USERNAME) FROM game_%d;", uint64(chatId))
	rows, err := th.Connection.Query(data)

	players := make([]Player, 0)
	if err != nil {
		log.Printf("Error. Query error: %s", err)
		return players
	}
	defer rows.Close()
	for rows.Next() {
		var (
			result     string
			userName   string
			userId     int64
			count      int
			money      float64
		)
		if err := rows.Scan(&result); err != nil {
			log.Fatal(err)
		}
		result = result[1:len(result) - 1]
		tokens := strings.FieldsFunc(result,
			func(c rune) bool {
				return c == ','
			})
		userId, _ = strconv.ParseInt(tokens[0], 10, 64)
		count, _ = strconv.Atoi(tokens[1])
		money, _ = strconv.ParseFloat(tokens[2], 64)
		userName = strings.TrimSuffix(strings.TrimPrefix(tokens[3],`"`), `"`)
		players = append(players, Player{
			UserName : userName,
			UserId   : userId,
			Count    : count,
			Money    : money,
		})
	}
	return players
}

func (th* Db) NewPlayer(chatId int64, userId int64, userName string, players int) bool {
	data := fmt.Sprintf(`INSERT INTO game_%d (USER_ID, USERNAME, PLAYERS, MONEY) VALUES($1, $2, $3, $4) 
	                    ON CONFLICT (USER_ID) DO UPDATE SET PLAYERS=game_%d.PLAYERS+$3;`, uint64(chatId), uint64(chatId))
	_, err := th.Connection.Exec(data, userId, userName, players, 0)
	if err != nil {
		log.Printf("Error. Can't add player: %s", err)
		return false
	}
	return true
}

func (th* Db) DropPlayer(chatId int64, userId int64, players int) {
	data := fmt.Sprintf(`UPDATE game_%d SET PLAYERS=game_%d.PLAYERS-$1 where USER_ID=$2;`, uint64(chatId), uint64(chatId))
	_, err := th.Connection.Exec(data, players, userId)
	if err != nil {
		log.Printf("Error. Can't remove player: %s", err)
		return
	}

	data = fmt.Sprintf(`DELETE FROM game_%d where PLAYERS <= 0 and USER_ID=$1;`, uint64(chatId))
	_, err = th.Connection.Exec(data, userId)
	if err != nil {
		log.Printf("Error. Can't remove row: %s", err)
	}
}

func (th* Db) PutMoney(chatId int64, userId int64, userName string, money float64) bool {
	data := fmt.Sprintf(`INSERT INTO game_%d (USER_ID, USERNAME, PLAYERS, MONEY) VALUES($1, $2, 1, $3) ON CONFLICT (USER_ID) DO UPDATE SET MONEY=game_%d.money+$3;`, uint64(chatId), uint64(chatId))
	_, err := th.Connection.Exec(data, userId, userName, money)
	if err != nil {
		log.Printf("Error. Can't add player: %s", err)
		return false
	}

	data = `INSERT INTO bank (chat_id, money, game_cost) VALUES($1, $2, 0.0) ON CONFLICT (chat_id) DO UPDATE SET MONEY=bank.money+$2;`
	_, err = th.Connection.Exec(data, chatId, money)
	if err != nil {
		log.Printf("Error. Can't money to the bank: %s", err)
	}
	return true
}

func (th* Db) Init() {
	data := `create table if not exists bank (chat_id int primary key, money real, game_cost real);`
	_, err := th.Connection.Exec(data)
	if err != nil {
		log.Printf("Error. Can't create bank table: %s", err)
	}
	data = `create table if not exists active_games (chat_id int primary key, game_holder text, game_holder_id int, holder_message text);`
	_, err = th.Connection.Exec(data)
	if err != nil {
		log.Printf("Error. Can't create active_games table: %s", err)
	}
}

func (th* Db) SetGameCost(chatId int64, howMuch float64) {
	data := `insert into bank (chat_id, money, game_cost) values($1, 0.0, $2) on conflict (chat_id) do update set game_cost=$2;`
	_, err := th.Connection.Exec(data, chatId, howMuch)
	if err != nil {
		log.Printf("Error. Can't set cost: %s", err)
	}
}

func (th* Db) PayForTheGame(chatId int64) {
	data := `update bank set money=(bank.money-(select (game_cost) from bank where chat_id=$1)) where chat_id=$1;`
	_, err := th.Connection.Exec(data, chatId)
	if err != nil {
		log.Printf("Error. Can't take money from the bank: %s", err)
	}
	if _, err := th.Connection.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS game_%d`, uint64(chatId))); err != nil {
		log.Println("Can't drop previous game")
	}
	data = fmt.Sprintf("delete from active_games where chat_id = %d;", chatId)
	if _, err := th.Connection.Exec(data); err != nil {
		log.Printf("Error. Can't delete active game: %s", err)
	}
}

func (th* Db) HowMuchMoney(chatId int64) float64 {
	data := fmt.Sprintf("select (money) from bank where chat_id=%d;", chatId)
	rows, err := th.Connection.Query(data)

	if err != nil {
		log.Printf("Error. Query error: %s", err)
		return 0.0
	}
	defer rows.Close()
	money := float64(0)
	var result string
	if rows.Next() {
		if err := rows.Scan(&result); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Sscanf(result, "%f", &money)
	return money
}

func (th* Db) GameExists(chatId int64) bool {
	tableId := fmt.Sprintf("game_%d", uint64(chatId))
	text := fmt.Sprintf("SELECT to_regclass('%s');", tableId)
	rows, err := th.Connection.Query(text)

	if err != nil {
		log.Printf("Error. Query error: %s", err)
		return false
	}
	defer rows.Close()
	var result string
	if rows.Next() {
		if err := rows.Scan(&result); err != nil {
			log.Printf("Cant find table: %s", err)
			return false
		}
	}
	if result == tableId {
		return true
	}
	return false
}

func DbConnect(host string, port string, user string, pswd string, name string, sslMode string) (*Db, error) {
	connection, err := sql.Open("postgres", fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", host, port, user, pswd, name, sslMode))
	return &Db{
		Connection : connection,
	}, err
}


