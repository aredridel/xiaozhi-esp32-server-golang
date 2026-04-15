package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// AudioStorage audio file storage utility
type AudioStorage struct {
	BasePath string
	MaxSize  int64
}

// NewAudioStorage creates an audio storage instance
func NewAudioStorage(basePath string, maxSize int64) *AudioStorage {
	// Ensure base directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		panic(fmt.Sprintf("Failed to create audio storage directory: %v", err))
	}

	return &AudioStorage{
		BasePath: basePath,
		MaxSize:  maxSize,
	}
}

// SaveAudioFile saves an audio file
// userID: user ID
// groupID: voiceprint group ID
// uuid: UUID identifier
// fileName: original file name
// fileData: file data
// Returns: file save path, file size, error
func (s *AudioStorage) SaveAudioFile(userID uint, groupID uint, uuid, fileName string, fileData io.Reader) (string, int64, error) {
	// Build storage path: storage/speakers/{user_id}/{group_id}/{uuid}.wav
	dirPath := filepath.Join(s.BasePath, fmt.Sprintf("%d", userID), fmt.Sprintf("%d", groupID))

	// Ensure directory exists
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", 0, fmt.Errorf("Failed to create directory: %v", err)
	}

	// Build file path (use UUID as filename, keep extension)
	ext := filepath.Ext(fileName)
	if ext == "" {
		ext = ".wav" // default extension
	}
	filePath := filepath.Join(dirPath, fmt.Sprintf("%s%s", uuid, ext))

	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to create file: %v", err)
	}
	defer file.Close()

	// Write file data (limit size)
	limitedReader := io.LimitReader(fileData, s.MaxSize)
	written, err := io.Copy(file, limitedReader)
	if err != nil {
		os.Remove(filePath) // Delete partially written file
		return "", 0, fmt.Errorf("Failed to write file: %v", err)
	}

	// Check file size
	if written >= s.MaxSize {
		os.Remove(filePath)
		return "", 0, fmt.Errorf("File size exceeds limit: %d bytes", s.MaxSize)
	}

	return filePath, written, nil
}

// SaveVoiceCloneAudioFile saves a voice clone audio file
func (s *AudioStorage) SaveVoiceCloneAudioFile(userID uint, uuid, fileName string, fileData io.Reader) (string, int64, error) {
	dirPath := filepath.Join(s.BasePath, "voice_clones", fmt.Sprintf("%d", userID))
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", 0, fmt.Errorf("Failed to create directory: %v", err)
	}

	ext := filepath.Ext(fileName)
	if ext == "" {
		ext = ".wav"
	}
	filePath := filepath.Join(dirPath, fmt.Sprintf("%s%s", uuid, ext))

	file, err := os.Create(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to create file: %v", err)
	}
	defer file.Close()

	limitedReader := io.LimitReader(fileData, s.MaxSize)
	written, err := io.Copy(file, limitedReader)
	if err != nil {
		os.Remove(filePath)
		return "", 0, fmt.Errorf("Failed to write file: %v", err)
	}
	if written >= s.MaxSize {
		os.Remove(filePath)
		return "", 0, fmt.Errorf("File size exceeds limit: %d bytes", s.MaxSize)
	}

	return filePath, written, nil
}

// DeleteAudioFile deletes an audio file
func (s *AudioStorage) DeleteAudioFile(filePath string) error {
	if filePath == "" {
		return nil
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // File does not exist, no need to delete
	}

	return os.Remove(filePath)
}

// GetAudioFile gets an audio file
func (s *AudioStorage) GetAudioFile(filePath string) (*os.File, error) {
	return os.Open(filePath)
}

// FileExists checks if file exists
func (s *AudioStorage) FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}
