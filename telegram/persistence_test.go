package telegram

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	// Run all tests
	m.Run()
}

func TestInsertMessage(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}
	defer db.Close()

	// Create table
	_, err = db.Exec(TABLE)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Prepare statements
	insertStmt, err := db.Prepare(INSERT)
	if err != nil {
		t.Fatalf("Failed to prepare insert statement: %v", err)
	}
	defer insertStmt.Close()

	// Test data
	msgId := 123
	channelId := int64(1001)
	date := time.Unix(1625648400, 0).UTC()
	user := "testuser"
	text := "Test message"
	msgType := "sent"

	// Insert message
	result, err := insertStmt.Exec(msgId, channelId, date, user, text, msgType)
	if err != nil {
		t.Fatalf("Failed to insert message: %v", err)
	}

	// Check if the message was inserted
	lastInsertId, _ := result.LastInsertId()
	if lastInsertId == 0 {
		t.Fatal("Expected a non-zero last insert ID")
	}

	// Query the message
	var id int
	var msgIdDB int
	var channelIdDB int64
	var dateDB time.Time
	var userDB string
	var textDB string
	var typeDB string

	err = db.QueryRow("SELECT id, msgId, channelId, date, user, text, type FROM messages WHERE id = ?", lastInsertId).Scan(
		&id, &msgIdDB, &channelIdDB, &dateDB, &userDB, &textDB, &typeDB,
	)
	if err != nil {
		t.Fatalf("Failed to query message: %v", err)
	}

	assert.Equal(t, msgId, msgIdDB)
	assert.Equal(t, channelId, channelIdDB)
	assert.Equal(t, date, dateDB)
	assert.Equal(t, user, userDB)
	assert.Equal(t, text, textDB)
	assert.Equal(t, msgType, typeDB)
}

func TestDeleteMessage(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}
	defer db.Close()

	// Create table
	_, err = db.Exec(TABLE)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Prepare statements
	insertStmt, err := db.Prepare(INSERT)
	if err != nil {
		t.Fatalf("Failed to prepare insert statement: %v", err)
	}
	defer insertStmt.Close()
	deleteStmt, err := db.Prepare(DELETE)
	if err != nil {
		t.Fatalf("Failed to prepare delete statement: %v - %s", t, err.Error())
	}
	defer deleteStmt.Close()

	// Insert test message
	msgId := 123
	channelId := int64(1001)
	date := int64(1625648400)
	user := "testuser"
	text := "Test message"
	msgType := "sent"

	result, err := insertStmt.Exec(msgId, channelId, date, user, text, msgType)
	if err != nil {
		t.Fatalf("Failed to insert message: %v", err)
	}

	lastInsertId, _ := result.LastInsertId()

	// Delete the message
	_, err = deleteStmt.Exec(lastInsertId)
	if err != nil {
		t.Fatalf("Failed to delete message: %v", err)
	}

	// Verify deletion
	var id int
	err = db.QueryRow("SELECT id FROM messages WHERE id = ?", lastInsertId).Scan(&id)
	if err == nil {
		t.Fatalf("Expected message to be deleted, but it was found with ID %d", id)
	}
}

func TestQueryMessages(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}
	defer db.Close()

	// Create table
	_, err = db.Exec(TABLE)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Prepare statements
	insertStmt, err := db.Prepare(INSERT)
	if err != nil {
		t.Fatalf("Failed to prepare insert statement: %v", err)
	}
	defer insertStmt.Close()

	// Insert multiple messages
	msgs := []struct {
		msgId     int
		channelId int64
		date      time.Time
		user      string
		text      string
		msgType   string
	}{
		{1, 1001, time.Unix(1625648400, 0).UTC(), "user1", "msg1", "sent"},
		{2, 1001, time.Unix(1625648401, 0).UTC(), "user2", "msg2", "received"},
		{3, 1002, time.Unix(1625648402, 0).UTC(), "user3", "msg3", "sent"},
	}

	for _, msg := range msgs {
		_, err := insertStmt.Exec(msg.msgId, msg.channelId, msg.date, msg.user, msg.text, msg.msgType)
		if err != nil {
			t.Fatalf("Failed to insert message: %v", err)
		}
	}

	// Query messages by channelId
	rows, err := db.Query("SELECT id, msgId, channelId, date, user, text, type FROM messages WHERE channelId = ?", 1001)
	if err != nil {
		t.Fatalf("Failed to query messages: %v", err)
	}
	defer rows.Close()

	var expectedMsgs []struct {
		id        int
		msgId     int
		channelId int64
		date      time.Time
		user      string
		text      string
		msgType   string
	}

	for i, msg := range msgs[:2] {
		expectedMsgs = append(expectedMsgs, struct {
			id        int
			msgId     int
			channelId int64
			date      time.Time
			user      string
			text      string
			msgType   string
		}{
			i + 1, msg.msgId, msg.channelId, msg.date, msg.user, msg.text, msg.msgType,
		})
	}

	var actualMsgs []struct {
		id        int
		msgId     int
		channelId int64
		date      time.Time
		user      string
		text      string
		msgType   string
	}

	for rows.Next() {
		var id int
		var msgId int
		var channelId int64
		var date time.Time
		var user string
		var text string
		var msgType string

		if err := rows.Scan(&id, &msgId, &channelId, &date, &user, &text, &msgType); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}

		actualMsgs = append(actualMsgs, struct {
			id        int
			msgId     int
			channelId int64
			date      time.Time
			user      string
			text      string
			msgType   string
		}{
			id, msgId, channelId, date, user, text, msgType,
		})
	}

	assert.Equal(t, expectedMsgs, actualMsgs)
}
