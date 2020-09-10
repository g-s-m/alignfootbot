package afdb

import (
	"database/sql"
	_ "github.com/lib/pq"
	"log"
	"fmt"
)
type Db struct {
	Connection *sql.DB
}

type Player struct {
	UserName string
	Count    int
	Money    float64
}

func (th *Db) Close() {
	th.Connection.Close()
}

func (th *Db) NewGame() {
	if _, err := th.Connection.Exec(`DROP TABLE IF EXISTS game`); err != nil {
		log.Panic("Can't drop previous game")
	}
	if _, err := th.Connection.Exec(`CREATE TABLE game (USER_ID INT PRIMARY KEY, USERNAME TEXT, CHAT_ID INT, PLAYERS INT, MONEY REAL);`); err != nil {
        log.Panic("Can't create table: %s", err)
	}
}

func (th *Db) ChatPlayers(chatId int64) []Player {
	data := fmt.Sprintf("SELECT (PLAYERS, MONEY, USERNAME) FROM game WHERE CHAT_ID = %d;", chatId)
	rows, err := th.Connection.Query(data)
	defer rows.Close()

	if err != nil {
		log.Printf("Error. Query error: %s", err)
	}
	players := make([]Player, 0)
	for rows.Next() {
		var (
			result     string
			userName   string
			count      int64
			money      float64
		)
		if err := rows.Scan(&result); err != nil {
			log.Fatal(err)
		}
		fmt.Sscanf(result[0:len(result)-1], "(%d,%f,%s)", &count, &money, &userName)
		players = append(players, Player{
			UserName : userName,
			Count    : int(count),
			Money    : money,
		})
	}
	return players
}

func (th* Db) NewPlayer(chatId int64, userId int64, userName string, players int) {
	data := `INSERT INTO game (USER_ID, USERNAME, CHAT_ID, PLAYERS, MONEY) VALUES($1, $2, $3, $4, $5) ON CONFLICT (USER_ID) DO UPDATE SET PLAYERS=game.PLAYERS+$4;`
	_, err := th.Connection.Exec(data, userId, userName, chatId, players, 0)
	if err != nil {
		log.Printf("Error. Can't add player: %s", err)
	}
}

func (th* Db) DropPlayer(chatId int64, userId int64, players int) {
	data := `UPDATE game SET PLAYERS=game.PLAYERS-$1 where USER_ID=$2 AND CHAT_ID=$3;`
	result, err := th.Connection.Exec(data, players, userId, chatId)
	if err != nil {
		log.Printf("Error. Can't remove player: %s", err)
	}
	ra, err := result.RowsAffected()
	log.Printf("Updated(inserted) %d rows", ra)

	data = `DELETE FROM game where PLAYERS <= 0 and USER_ID=$1;`
	result, err = th.Connection.Exec(data, userId)
	if err != nil {
		log.Printf("Error. Can't remove row: %s", err)
	}
}

func (th* Db) PutMoney(chatId int64, userId int64, userName string, money float64) {
	data := `INSERT INTO game (USER_ID, USERNAME, CHAT_ID, PLAYERS, MONEY) VALUES($1, $2, $3, 1, $4) ON CONFLICT (USER_ID) DO UPDATE SET MONEY=game.money+$4;`
	_, err := th.Connection.Exec(data, userId, userName, chatId, money)
	if err != nil {
		log.Printf("Error. Can't add player: %s", err)
	}

	data = `INSERT INTO bank (chat_id, money, game_cost) VALUES($1, $2, 0.0) ON CONFLICT (chat_id) DO UPDATE SET MONEY=bank.money+$2;`
	_, err = th.Connection.Exec(data, chatId, money)
	if err != nil {
		log.Printf("Error. Can't money to the bank: %s", err)
	}
}

func (th* Db) CreateMoneyTable() {
	data := `create table if not exists bank (chat_id int primary key, money real, game_cost real);`
	_, err := th.Connection.Exec(data)
	if err != nil {
		log.Printf("Error. Can't create bank table: %s", err)
	}
}

func (th* Db) SetGameCost(chatId int64, howMuch float64) {
	data := `insert into bank (chat_id, money, game_cost) values($1, 0.0, $2) on conflict (chat_id) do update set game_cost=$2;`
	_, err := th.Connection.Exec(data, chatId, howMuch)
	if err != nil {
		log.Printf("Error. Can't set cost: %s", err)
	}
}

func (th* Db) TakeMoney(chatId int64, howMuch float64) {
	data := `update bank set money=bank.money-$1 where chat_id = $2;`
	_, err := th.Connection.Exec(data, howMuch, chatId)
	if err != nil {
		log.Printf("Error. Can't take money from the bank: %s", err)
	}
}

func (th* Db) HowMuchMoney(chatId int64) float64 {
	data := fmt.Sprintf("select (money) from bank where chat_id=%d;", chatId)
	rows, err := th.Connection.Query(data)
	defer rows.Close()

	if err != nil {
		log.Printf("Error. Query error: %s", err)
	}
	money := float64(0)
	var result string
	if rows.Next() {
		if err := rows.Scan(&result); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Sscanf(result, "(%f)", &money)
	return money
}

func DbConnect(host string, port string, user string, pswd string, name string, sslMode string) (*Db, error) {
	connection, err := sql.Open("postgres", fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", host, port, user, pswd, name, sslMode))
	return &Db{
		Connection : connection,
	}, err
}


