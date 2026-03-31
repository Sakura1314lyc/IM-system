package main

import (
	"crypto/tls"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

type Database struct {
	db *sql.DB
}

type DBUser struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	Avatar    string    `json:"avatar"`
	CreatedAt time.Time `json:"created_at"`
}

type DBGroup struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	CreatorID   int       `json:"creator_id"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type DBGroupMember struct {
	ID      int `json:"id"`
	GroupID int `json:"group_id"`
	UserID  int `json:"user_id"`
}

type DBMessage struct {
	ID        int       `json:"id"`
	FromID    int       `json:"from_id"`
	ToID      *int      `json:"to_id,omitempty"`    // for private messages
	GroupID   *int      `json:"group_id,omitempty"` // for group messages
	Content   string    `json:"content"`
	Type      string    `json:"type"` // "public", "private", "group"
	CreatedAt time.Time `json:"created_at"`
}

func InitDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	database := &Database{db: db}

	// Create tables
	if err := database.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	// Create default users
	if err := database.createDefaultUsers(); err != nil {
		log.Printf("Warning: failed to create default users: %v", err)
	}

	return database, nil
}

func (d *Database) createTables() error {
	// Users table
	userTable := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		avatar TEXT DEFAULT '👤',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	// Groups table
	groupTable := `
	CREATE TABLE IF NOT EXISTS groups (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		creator_id INTEGER NOT NULL,
		description TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (creator_id) REFERENCES users(id)
	);`

	// Group members table
	groupMembersTable := `
	CREATE TABLE IF NOT EXISTS group_members (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		group_id INTEGER NOT NULL,
		user_id INTEGER NOT NULL,
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(group_id, user_id),
		FOREIGN KEY (group_id) REFERENCES groups(id),
		FOREIGN KEY (user_id) REFERENCES users(id)
	);`

	// Messages table
	messagesTable := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		from_id INTEGER NOT NULL,
		to_id INTEGER,
		group_id INTEGER,
		content TEXT NOT NULL,
		type TEXT NOT NULL CHECK(type IN ('public', 'private', 'group')),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (from_id) REFERENCES users(id),
		FOREIGN KEY (to_id) REFERENCES users(id),
		FOREIGN KEY (group_id) REFERENCES groups(id)
	);`

	tables := []string{userTable, groupTable, groupMembersTable, messagesTable}

	for _, table := range tables {
		if _, err := d.db.Exec(table); err != nil {
			return fmt.Errorf("failed to create table: %v", err)
		}
	}

	return nil
}

func (d *Database) createDefaultUsers() error {
	defaultUsers := []struct {
		username string
		password string
		avatar   string
	}{
		{"alice", "123", "👨‍💼"},
		{"bob", "123", "👩‍💼"},
		{"charlie", "123", "👨‍💻"},
	}

	for _, user := range defaultUsers {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password for %s: %v", user.username, err)
		}

		_, err = d.db.Exec(`
			INSERT OR IGNORE INTO users (username, password, avatar)
			VALUES (?, ?, ?)`,
			user.username, string(hashedPassword), user.avatar)
		if err != nil {
			return fmt.Errorf("failed to create default user %s: %v", user.username, err)
		}
	}

	return nil
}

func (d *Database) AuthenticateUser(username, password string) (*DBUser, error) {
	var user DBUser
	err := d.db.QueryRow(`
		SELECT id, username, password, avatar, created_at
		FROM users WHERE username = ?`, username).Scan(
		&user.ID, &user.Username, &user.Password, &user.Avatar, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("database error: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid password")
	}

	return &user, nil
}

func (d *Database) RegisterUser(username, password, avatar string) error {
	if len(username) < 3 || len(username) > 20 {
		return fmt.Errorf("username must be 3-20 characters")
	}
	if len(password) < 6 {
		return fmt.Errorf("password must be at least 6 characters")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}

	_, err = d.db.Exec(`
		INSERT INTO users (username, password, avatar)
		VALUES (?, ?, ?)`,
		username, string(hashedPassword), avatar)

	if err != nil {
		if sqliteErr, ok := err.(sqlite3.Error); ok && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return fmt.Errorf("username already exists")
		}
		return fmt.Errorf("failed to register user: %v", err)
	}

	return nil
}

func (d *Database) GetUserByID(id int) (*DBUser, error) {
	var user DBUser
	err := d.db.QueryRow(`
		SELECT id, username, password, avatar, created_at
		FROM users WHERE id = ?`, id).Scan(
		&user.ID, &user.Username, &user.Password, &user.Avatar, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("database error: %v", err)
	}

	return &user, nil
}

func (d *Database) GetUserByUsername(username string) (*DBUser, error) {
	var user DBUser
	err := d.db.QueryRow(`
		SELECT id, username, password, avatar, created_at
		FROM users WHERE username = ?`, username).Scan(
		&user.ID, &user.Username, &user.Password, &user.Avatar, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("database error: %v", err)
	}

	return &user, nil
}

func (d *Database) CreateGroup(name string, creatorID int, description string) (*DBGroup, error) {
	result, err := d.db.Exec(`
		INSERT INTO groups (name, creator_id, description)
		VALUES (?, ?, ?)`,
		name, creatorID, description)

	if err != nil {
		if sqliteErr, ok := err.(sqlite3.Error); ok && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return nil, fmt.Errorf("group name already exists")
		}
		return nil, fmt.Errorf("failed to create group: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get group id: %v", err)
	}

	// Add creator as first member
	_, err = d.db.Exec(`
		INSERT INTO group_members (group_id, user_id)
		VALUES (?, ?)`, id, creatorID)
	if err != nil {
		return nil, fmt.Errorf("failed to add creator to group: %v", err)
	}

	group := &DBGroup{
		ID:          int(id),
		Name:        name,
		CreatorID:   creatorID,
		Description: description,
		CreatedAt:   time.Now(),
	}

	return group, nil
}

func (d *Database) JoinGroup(groupName string, userID int) error {
	var groupID int
	err := d.db.QueryRow("SELECT id FROM groups WHERE name = ?", groupName).Scan(&groupID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("group not found")
	}
	if err != nil {
		return fmt.Errorf("database error: %v", err)
	}

	_, err = d.db.Exec(`
		INSERT OR IGNORE INTO group_members (group_id, user_id)
		VALUES (?, ?)`, groupID, userID)

	if err != nil {
		return fmt.Errorf("failed to join group: %v", err)
	}

	return nil
}

func (d *Database) LeaveGroup(groupName string, userID int) error {
	var groupID int
	err := d.db.QueryRow("SELECT id FROM groups WHERE name = ?", groupName).Scan(&groupID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("group not found")
	}
	if err != nil {
		return fmt.Errorf("database error: %v", err)
	}

	result, err := d.db.Exec(`
		DELETE FROM group_members
		WHERE group_id = ? AND user_id = ?`, groupID, userID)

	if err != nil {
		return fmt.Errorf("failed to leave group: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user is not a member of this group")
	}

	return nil
}

func (d *Database) GetGroupMembers(groupName string) ([]*DBUser, error) {
	rows, err := d.db.Query(`
		SELECT u.id, u.username, u.password, u.avatar, u.created_at
		FROM users u
		JOIN group_members gm ON u.id = gm.user_id
		JOIN groups g ON gm.group_id = g.id
		WHERE g.name = ?`, groupName)

	if err != nil {
		return nil, fmt.Errorf("database error: %v", err)
	}
	defer rows.Close()

	var members []*DBUser
	for rows.Next() {
		var user DBUser
		err := rows.Scan(&user.ID, &user.Username, &user.Password, &user.Avatar, &user.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %v", err)
		}
		members = append(members, &user)
	}

	return members, nil
}

func (d *Database) SaveMessage(fromID int, toID *int, groupID *int, content, msgType string) (*DBMessage, error) {
	result, err := d.db.Exec(`
		INSERT INTO messages (from_id, to_id, group_id, content, type)
		VALUES (?, ?, ?, ?, ?)`,
		fromID, toID, groupID, content, msgType)

	if err != nil {
		return nil, fmt.Errorf("failed to save message: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get message id: %v", err)
	}

	message := &DBMessage{
		ID:        int(id),
		FromID:    fromID,
		ToID:      toID,
		GroupID:   groupID,
		Content:   content,
		Type:      msgType,
		CreatedAt: time.Now(),
	}

	return message, nil
}

func (d *Database) GetMessageHistory(limit int) ([]*DBMessage, error) {
	rows, err := d.db.Query(`
		SELECT id, from_id, to_id, group_id, content, type, created_at
		FROM messages
		ORDER BY created_at DESC
		LIMIT ?`, limit)

	if err != nil {
		return nil, fmt.Errorf("database error: %v", err)
	}
	defer rows.Close()

	var messages []*DBMessage
	for rows.Next() {
		var msg DBMessage
		err := rows.Scan(&msg.ID, &msg.FromID, &msg.ToID, &msg.GroupID, &msg.Content, &msg.Type, &msg.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %v", err)
		}
		messages = append(messages, &msg)
	}

	return messages, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

// generateSelfSignedCert generates a self-signed TLS certificate for development
func generateSelfSignedCert() (tls.Certificate, error) {
	// For development purposes, we'll try to load existing certs
	// In production, use proper certificates
	cert, err := tls.LoadX509KeyPair("server.crt", "server.key")
	if err != nil {
		// If certs don't exist, return an error
		return tls.Certificate{}, fmt.Errorf("TLS certificates not found. Please generate server.crt and server.key for production use")
	}
	return cert, nil
}
