package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

// ChunkStorageAdapter defines the storage pattern interface for chunked uploads
type ChunkStorageAdapter interface {
	StoreChunk(uploadID string, chunkIdx int, src io.Reader) error
	GetUploadedChunks(uploadID string) ([]int, error)
	AssembleAndSave(ctx context.Context, uploadID string, destPath string) (int64, error)
	Cleanup(uploadID string) error
}

// FileChunkStorageAdapter implements disk-based chunk storage
type FileChunkStorageAdapter struct {
	baseUploadDir string
	sessions      map[string]*UploadSession
	mu            *sync.RWMutex
}

func NewFileChunkStorageAdapter(baseUploadDir string, sessions map[string]*UploadSession, mu *sync.RWMutex) *FileChunkStorageAdapter {
	return &FileChunkStorageAdapter{
		baseUploadDir: baseUploadDir,
		sessions:      sessions,
		mu:            mu,
	}
}

func (a *FileChunkStorageAdapter) StoreChunk(uploadID string, chunkIdx int, src io.Reader) error {
	a.mu.RLock()
	session, exists := a.sessions[uploadID]
	a.mu.RUnlock()

	chunkDir := filepath.Join(a.baseUploadDir, uploadID)
	if exists && session.ChunkDir != "" {
		chunkDir = session.ChunkDir
	}

	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		return fmt.Errorf("mkdir chunk dir: %w", err)
	}

	chunkPath := filepath.Join(chunkDir, strconv.Itoa(chunkIdx))
	destFile, err := os.Create(chunkPath)
	if err != nil {
		return fmt.Errorf("create chunk file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, src); err != nil {
		return fmt.Errorf("write chunk file: %w", err)
	}

	if exists {
		if stat, err := os.Stat(chunkPath); err == nil {
			a.mu.Lock()
			session.BytesSinceLastCheck += stat.Size()
			a.mu.Unlock()
		}
	}

	return nil
}

func (a *FileChunkStorageAdapter) GetUploadedChunks(uploadID string) ([]int, error) {
	chunkDir := filepath.Join(a.baseUploadDir, uploadID)
	entries, err := os.ReadDir(chunkDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []int{}, nil
		}
		return nil, err
	}

	var chunks []int
	for _, entry := range entries {
		if !entry.IsDir() {
			if idx, err := strconv.Atoi(entry.Name()); err == nil {
				chunks = append(chunks, idx)
			}
		}
	}
	sort.Ints(chunks)
	return chunks, nil
}

func (a *FileChunkStorageAdapter) AssembleAndSave(ctx context.Context, uploadID string, destPath string) (int64, error) {
	chunkDir := filepath.Join(a.baseUploadDir, uploadID)
	entries, err := os.ReadDir(chunkDir)
	if err != nil {
		return 0, fmt.Errorf("read chunk dir: %w", err)
	}

	var chunkFiles []string
	for _, entry := range entries {
		if !entry.IsDir() {
			if _, err := strconv.Atoi(entry.Name()); err == nil {
				chunkFiles = append(chunkFiles, entry.Name())
			}
		}
	}

	sort.Slice(chunkFiles, func(i, j int) bool {
		valI, _ := strconv.Atoi(chunkFiles[i])
		valJ, _ := strconv.Atoi(chunkFiles[j])
		return valI < valJ
	})

	if len(chunkFiles) == 0 {
		return 0, fmt.Errorf("no chunk files found to assemble")
	}

	destFile, err := os.Create(destPath)
	if err != nil {
		return 0, fmt.Errorf("create destination file: %w", err)
	}
	defer destFile.Close()

	for _, filename := range chunkFiles {
		chunkPath := filepath.Join(chunkDir, filename)
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			return 0, fmt.Errorf("open chunk %s: %w", filename, err)
		}
		_, err = io.Copy(destFile, chunkFile)
		chunkFile.Close()
		if err != nil {
			return 0, fmt.Errorf("append chunk %s: %w", filename, err)
		}
	}

	stat, err := destFile.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat merged file: %w", err)
	}

	return stat.Size(), nil
}

func (a *FileChunkStorageAdapter) Cleanup(uploadID string) error {
	chunkDir := filepath.Join(a.baseUploadDir, uploadID)
	return os.RemoveAll(chunkDir)
}

// InMemoryChunkStorageAdapter implements TTL-based in-memory map cache storage
type InMemoryChunkItem struct {
	Data      []byte
	CreatedAt time.Time
}

type InMemoryChunkStorageAdapter struct {
	cache    map[string]map[int]*InMemoryChunkItem // uploadID -> chunkIdx -> chunkItem
	ttl      time.Duration
	mu       sync.RWMutex
	sessions map[string]*UploadSession
	sessMu   *sync.RWMutex
}

func NewInMemoryChunkStorageAdapter(ttl time.Duration, sessions map[string]*UploadSession, sessMu *sync.RWMutex) *InMemoryChunkStorageAdapter {
	adapter := &InMemoryChunkStorageAdapter{
		cache:    make(map[string]map[int]*InMemoryChunkItem),
		ttl:      ttl,
		sessions: sessions,
		sessMu:   sessMu,
	}

	// Start background cleanup ticker for TTL expiration
	go adapter.startTTLCleaner(1 * time.Minute)

	return adapter
}

func (a *InMemoryChunkStorageAdapter) startTTLCleaner(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		a.mu.Lock()
		now := time.Now()
		for uploadID, chunks := range a.cache {
			for idx, item := range chunks {
				if now.Sub(item.CreatedAt) > a.ttl {
					delete(chunks, idx)
				}
			}
			if len(chunks) == 0 {
				delete(a.cache, uploadID)
			}
		}
		a.mu.Unlock()
	}
}

func (a *InMemoryChunkStorageAdapter) StoreChunk(uploadID string, chunkIdx int, src io.Reader) error {
	data, err := io.ReadAll(src)
	if err != nil {
		return fmt.Errorf("read chunk reader: %w", err)
	}

	a.mu.Lock()
	if _, exists := a.cache[uploadID]; !exists {
		a.cache[uploadID] = make(map[int]*InMemoryChunkItem)
	}
	a.cache[uploadID][chunkIdx] = &InMemoryChunkItem{
		Data:      data,
		CreatedAt: time.Now(),
	}
	a.mu.Unlock()

	a.sessMu.RLock()
	session, exists := a.sessions[uploadID]
	a.sessMu.RUnlock()

	if exists {
		a.sessMu.Lock()
		session.BytesSinceLastCheck += int64(len(data))
		a.sessMu.Unlock()
	}

	return nil
}

func (a *InMemoryChunkStorageAdapter) GetUploadedChunks(uploadID string) ([]int, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	chunksMap, exists := a.cache[uploadID]
	if !exists {
		return []int{}, nil
	}

	var chunks []int
	for idx := range chunksMap {
		chunks = append(chunks, idx)
	}
	sort.Ints(chunks)
	return chunks, nil
}

func (a *InMemoryChunkStorageAdapter) AssembleAndSave(ctx context.Context, uploadID string, destPath string) (int64, error) {
	a.mu.RLock()
	chunksMap, exists := a.cache[uploadID]
	if !exists || len(chunksMap) == 0 {
		a.mu.RUnlock()
		return 0, fmt.Errorf("no in-memory chunks found for upload session %s", uploadID)
	}

	var chunkIndices []int
	for idx := range chunksMap {
		chunkIndices = append(chunkIndices, idx)
	}
	sort.Ints(chunkIndices)

	// Make a shallow copy of pointers to release read lock quickly
	orderedChunks := make([][]byte, len(chunkIndices))
	for i, idx := range chunkIndices {
		orderedChunks[i] = chunksMap[idx].Data
	}
	a.mu.RUnlock()

	destFile, err := os.Create(destPath)
	if err != nil {
		return 0, fmt.Errorf("create destination file: %w", err)
	}
	defer destFile.Close()

	var totalWritten int64
	for _, chunkBytes := range orderedChunks {
		n, err := destFile.Write(chunkBytes)
		if err != nil {
			return 0, fmt.Errorf("write in-memory chunk to file: %w", err)
		}
		totalWritten += int64(n)
	}

	return totalWritten, nil
}

func (a *InMemoryChunkStorageAdapter) Cleanup(uploadID string) error {
	a.mu.Lock()
	delete(a.cache, uploadID)
	a.mu.Unlock()
	return nil
}
