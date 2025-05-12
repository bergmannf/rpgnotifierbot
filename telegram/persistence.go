// This file will maintain a list of all received and sent messages.
package telegram

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type MessageType = string

type DBMessage struct {
	Id        *int
	MsgId     *int
	ChannelId *int64
	Date      *time.Time
	User      *string
	Text      *string
	Type      *MessageType
}

const SENT MessageType = "sent"
const RECEIVED MessageType = "received"

const TABLE string = `CREATE TABLE IF NOT EXISTS messages (
id INTEGER NOT NULL PRIMARY KEY,
msgId INTEGER NOT NULL,
channelId INTEGER NOT NULL,
date DATETIME NOT NULL,
user TEXT,
text TEXT,
type TEXT CHECK (type in ('sent', 'received')) NOT NULL DEFAULT 'received'
)`
const INSERT string = `INSERT INTO messages VALUES(NULL, ?, ?, ?, ?, ?, ?)`
const DELETE string = `DELETE FROM messages WHERE id = ?`
const SENT_QUERY string = `SELECT * FROM messages WHERE type = 'sent' AND channelId = ?`
const RECEIVED_QUERY string = `SELECT * FROM messages WHERE type = 'received' AND channelId = ?`

type MessageDB struct {
	connection                     *sql.DB
	insert, delete, sent, received *sql.Stmt
}

func OpenDatabase(dbPath string) (*MessageDB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	_, err = conn.Exec(TABLE)
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
	sent, err := conn.Prepare(SENT_QUERY)
	if err != nil {
		return nil, err
	}
	received, err := conn.Prepare(RECEIVED_QUERY)
	if err != nil {
		return nil, err
	}
	return &MessageDB{
		connection: conn,
		insert:     ins,
		delete:     del,
		sent:       sent,
		received:   received,
	}, nil
}
