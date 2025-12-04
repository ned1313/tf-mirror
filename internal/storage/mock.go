package storage

import (
	"context"
	"io"
	"time"
)

// MockStorage implements Storage interface for testing purposes
// It provides a simple in-memory storage with configurable behavior
type MockStorage struct {
	data          map[string][]byte
	PresignedBase string // Base URL for presigned URLs
}

// NewMockStorage creates a new mock storage instance
func NewMockStorage() *MockStorage {
	return &MockStorage{
		data:          make(map[string][]byte),
		PresignedBase: "http://example.com/storage",
	}
}

// Upload stores data in memory
func (m *MockStorage) Upload(ctx context.Context, key string, reader io.Reader, contentType string, metadata map[string]string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	m.data[key] = data
	return nil
}

// Download returns stored data
func (m *MockStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	data, ok := m.data[key]
	if !ok {
		return nil, io.EOF
	}
	return io.NopCloser(&sectionReader{data: data}), nil
}

// Delete removes data from memory
func (m *MockStorage) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

// Exists checks if data exists
func (m *MockStorage) Exists(ctx context.Context, key string) (bool, error) {
	_, ok := m.data[key]
	return ok, nil
}

// GetPresignedURL returns a mock presigned URL
func (m *MockStorage) GetPresignedURL(ctx context.Context, key string, expiration time.Duration) (string, error) {
	return m.PresignedBase + "/" + key, nil
}

// GetMetadata returns empty metadata
func (m *MockStorage) GetMetadata(ctx context.Context, key string) (map[string]string, error) {
	return make(map[string]string), nil
}

// ListObjects returns stored keys with prefix
func (m *MockStorage) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	for key := range m.data {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

// GetObjectSize returns the size of stored data
func (m *MockStorage) GetObjectSize(ctx context.Context, key string) (int64, error) {
	data, ok := m.data[key]
	if !ok {
		return 0, nil
	}
	return int64(len(data)), nil
}

// Close does nothing for mock storage
func (m *MockStorage) Close() error {
	return nil
}

// SetData allows tests to pre-populate storage with data
func (m *MockStorage) SetData(key string, data []byte) {
	m.data[key] = data
}

// GetData allows tests to inspect stored data
func (m *MockStorage) GetData(key string) ([]byte, bool) {
	data, ok := m.data[key]
	return data, ok
}

// sectionReader helper for Download
type sectionReader struct {
	data []byte
	pos  int64
}

func (s *sectionReader) Read(p []byte) (n int, err error) {
	if s.pos >= int64(len(s.data)) {
		return 0, io.EOF
	}
	n = copy(p, s.data[s.pos:])
	s.pos += int64(n)
	return n, nil
}

// Compile-time check to ensure MockStorage implements Storage interface
var _ Storage = (*MockStorage)(nil)
