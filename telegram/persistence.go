// This file will maintain a list of all received and sent messages.
package telegram

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

type DBMessage struct {
	Id *int
	MsgId *int
	ChannelId *int64
	Date *int64
	User *string
	Text *string
	Type *string
}

const TABLE string = `CREATE TABLE IF NOT EXISTS messages (
id INTEGER NOT NULL PRIMARY KEY,
msgId INTEGER NOT NULL,
channelId INTEGER NOT NULL,
date DATETIME NOT NULL,
user TEXT,
text TEXT,
type TEXT CHECK (type in ('sent', 'received') NOT NULL DEFAULT 'received')
)`
const INSERT string = `INSERT INTO messages VALUES(NULL, ?, ?, ?, ?, ?, ?)`
const DELETE string = `DELETE FROM messages WHERE id = ?`
const QUERY string = `SELECT * FROM messages WHERE channelId = ?`

type MessageDB struct {
	connection            *sql.DB
	insert, delete, query *sql.Stmt
}

func OpenDatabase(dbPath string) (*MessageDB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	ins, err := conn.Prepare(INSERT)
	if err != nil {
		return nil, err
	}
	del, err := conn.Prepare(DELETE)
	if err != nil {
		return nil, err
	}
	quer, err := conn.Prepare(QUERY)
	if err != nil {
		return nil, err
	}
	return &MessageDB{
		connection: conn,
		insert:     ins,
		delete:     del,
		query:      quer,
	}, nil
}
