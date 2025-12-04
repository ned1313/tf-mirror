package database

import (
	"database/sql"
	"time"
)

// Provider represents a cached Terraform provider
type Provider struct {
	ID        int64
	Namespace string
	Type      string
	Version   string
	Platform  string

	// Provider metadata
	Filename    string
	DownloadURL string
	Shasum      string
	SigningKeys sql.NullString

	// Storage information
	S3Key     string
	SizeBytes int64

	// Status flags
	Deprecated bool
	Blocked    bool

	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AdminUser represents an administrator account
type AdminUser struct {
	ID           int64
	Username     string
	PasswordHash string

	// User metadata
	FullName sql.NullString
	Email    sql.NullString

	// Status
	Active bool

	// Timestamps
	CreatedAt   time.Time
	UpdatedAt   time.Time
	LastLoginAt sql.NullTime
}

// AdminSession represents an active admin session
type AdminSession struct {
	ID       int64
	UserID   int64
	TokenJTI string

	// Session metadata
	IPAddress sql.NullString
	UserAgent sql.NullString

	// Expiration
	ExpiresAt time.Time
	Revoked   bool

	// Timestamps
	CreatedAt time.Time
}

// AdminAction represents an audit log entry
type AdminAction struct {
	ID     int64
	UserID sql.NullInt64

	// Action details
	Action       string
	ResourceType string
	ResourceID   sql.NullString

	// Request context
	IPAddress sql.NullString
	UserAgent sql.NullString

	// Action result
	Success      bool
	ErrorMessage sql.NullString

	// Additional data (JSON)
	Metadata sql.NullString

	// Timestamp
	CreatedAt time.Time
}

// DownloadJob represents a provider download job
type DownloadJob struct {
	ID     int64
	UserID sql.NullInt64

	// Job configuration
	SourceType string // 'hcl', 'api'
	SourceData string

	// Job status
	Status   string // pending, running, completed, failed
	Progress int    // percentage 0-100

	// Results
	TotalItems     int
	CompletedItems int
	FailedItems    int

	// Error handling
	ErrorMessage sql.NullString

	// Timestamps
	CreatedAt   time.Time
	StartedAt   sql.NullTime
	CompletedAt sql.NullTime
}

// DownloadJobItem represents a single item in a download job
type DownloadJobItem struct {
	ID    int64
	JobID int64

	// Item details
	Namespace string
	Type      string
	Version   string
	Platform  string

	// Item status
	Status string // pending, downloading, completed, failed

	// Progress
	DownloadURL     sql.NullString
	SizeBytes       sql.NullInt64
	DownloadedBytes sql.NullInt64

	// Results
	ProviderID   sql.NullInt64
	ErrorMessage sql.NullString

	// Retry tracking
	RetryCount int

	// Timestamps
	CreatedAt   time.Time
	StartedAt   sql.NullTime
	CompletedAt sql.NullTime
}
