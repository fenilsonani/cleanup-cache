package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// HashFile computes SHA256 hash of a file
func HashFile(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// HashFileQuick computes SHA256 hash of first and last chunks of a file
// This is faster for large files and good enough for duplicate detection
func HashFileQuick(filepath string, chunkSize int64) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", err
	}

	fileSize := fileInfo.Size()
	hash := sha256.New()

	// If file is smaller than 2*chunkSize, hash the whole file
	if fileSize <= chunkSize*2 {
		if _, err := io.Copy(hash, file); err != nil {
			return "", err
		}
		return hex.EncodeToString(hash.Sum(nil)), nil
	}

	// Hash first chunk
	firstChunk := make([]byte, chunkSize)
	if _, err := file.Read(firstChunk); err != nil {
		return "", err
	}
	hash.Write(firstChunk)

	// Hash last chunk
	if _, err := file.Seek(-chunkSize, io.SeekEnd); err != nil {
		return "", err
	}
	lastChunk := make([]byte, chunkSize)
	if _, err := file.Read(lastChunk); err != nil {
		return "", err
	}
	hash.Write(lastChunk)

	return hex.EncodeToString(hash.Sum(nil)), nil
}
