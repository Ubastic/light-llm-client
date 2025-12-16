package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite database connection
type DB struct {
	conn *sql.DB
}

// New creates a new database connection
func New(dbPath string) (*DB, error) {
	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database connection
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings
	conn.SetMaxOpenConns(1) // SQLite works best with single connection
	conn.SetMaxIdleConns(1)

	db := &DB{conn: conn}

	// Run migrations
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// migrate runs database migrations
func (db *DB) migrate() error {
	migrations := []string{
		// Conversations table
		`CREATE TABLE IF NOT EXISTS conversations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			category TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Messages table
		`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id INTEGER NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			original_content TEXT DEFAULT '',
			provider TEXT DEFAULT '',
			model TEXT DEFAULT '',
			attachments TEXT DEFAULT '',
			tokens_used INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
		)`,

		// Settings table
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// FTS5 virtual table for full-text search
		`CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
			content,
			conversation_id UNINDEXED,
			content=messages,
			content_rowid=id
		)`,

		// Triggers to keep FTS in sync
		`CREATE TRIGGER IF NOT EXISTS messages_ai AFTER INSERT ON messages BEGIN
			INSERT INTO messages_fts(rowid, content, conversation_id)
			VALUES (new.id, new.content, new.conversation_id);
		END`,

		`CREATE TRIGGER IF NOT EXISTS messages_ad AFTER DELETE ON messages BEGIN
			DELETE FROM messages_fts WHERE rowid = old.id;
		END`,

		`CREATE TRIGGER IF NOT EXISTS messages_au AFTER UPDATE ON messages BEGIN
			UPDATE messages_fts SET content = new.content WHERE rowid = new.id;
		END`,

		// Indexes for better performance
		`CREATE INDEX IF NOT EXISTS idx_messages_conversation_id ON messages(conversation_id)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_conversation_created ON messages(conversation_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_conversations_updated_at ON conversations(updated_at DESC)`,
	}

	for _, migration := range migrations {
		if _, err := db.conn.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, migration)
		}
	}

	// Run additional migrations for existing databases
	if err := db.runAdditionalMigrations(); err != nil {
		return fmt.Errorf("additional migration failed: %w", err)
	}

	return nil
}

// runAdditionalMigrations runs migrations for existing databases
func (db *DB) runAdditionalMigrations() error {
	// Check if original_content column exists
	var columnExists bool
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('messages') WHERE name = 'original_content'
	`).Scan(&columnExists)
	if err != nil {
		return fmt.Errorf("failed to check if original_content column exists: %w", err)
	}

	// If column doesn't exist, add it
	if !columnExists {
		if _, err := db.conn.Exec(`ALTER TABLE messages ADD COLUMN original_content TEXT DEFAULT ''`); err != nil {
			return fmt.Errorf("failed to add original_content column: %w", err)
		}
		fmt.Println("Added original_content column to messages table")
	}

	return nil
}

// DBStats represents database statistics
type DBStats struct {
	ConversationCount int64
	MessageCount      int64
	DBSizeBytes       int64
}

// GetStats returns database statistics
func (db *DB) GetStats() (*DBStats, error) {
	stats := &DBStats{}
	
	// Get conversation count
	err := db.conn.QueryRow("SELECT COUNT(*) FROM conversations").Scan(&stats.ConversationCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count conversations: %w", err)
	}
	
	// Get message count
	err = db.conn.QueryRow("SELECT COUNT(*) FROM messages").Scan(&stats.MessageCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count messages: %w", err)
	}
	
	// Get database size (page_count * page_size)
	var pageCount, pageSize int64
	err = db.conn.QueryRow("PRAGMA page_count").Scan(&pageCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get page count: %w", err)
	}
	
	err = db.conn.QueryRow("PRAGMA page_size").Scan(&pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get page size: %w", err)
	}
	
	stats.DBSizeBytes = pageCount * pageSize
	
	return stats, nil
}

// Vacuum optimizes the database file
func (db *DB) Vacuum() error {
	_, err := db.conn.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}
	return nil
}
