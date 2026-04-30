package server

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func saveAvatarFile(uploadDir, username, avatarData string) (string, error) {
	// Already a URL path, return as-is
	if strings.HasPrefix(avatarData, "/") {
		return avatarData, nil
	}

	// Not a data URI (emoji or text), return as-is
	if !strings.HasPrefix(avatarData, "data:") {
		return avatarData, nil
	}

	// Parse data URI: data:image/png;base64,iVBOR...
	commaIdx := strings.Index(avatarData, ",")
	if commaIdx < 0 {
		return "", fmt.Errorf("invalid data URI")
	}

	mimePart := avatarData[:commaIdx]
	base64Data := avatarData[commaIdx+1:]

	// Determine extension from MIME type
	var ext string
	switch {
	case strings.Contains(mimePart, "image/png"):
		ext = ".png"
	case strings.Contains(mimePart, "image/jpeg"):
		ext = ".jpg"
	case strings.Contains(mimePart, "image/gif"):
		ext = ".gif"
	case strings.Contains(mimePart, "image/webp"):
		ext = ".webp"
	default:
		return "", fmt.Errorf("unsupported image type: %s", mimePart)
	}

	// Decode base64 data
	decoded, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		// Try with padding
		decoded, err = base64.StdEncoding.WithPadding(base64.NoPadding).DecodeString(base64Data)
		if err != nil {
			return "", fmt.Errorf("failed to decode image data: %w", err)
		}
	}

	// Create upload directory
	avatarDir := filepath.Join(uploadDir, "avatars")
	if err := os.MkdirAll(avatarDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create avatar directory: %w", err)
	}

	// Remove old avatar files for this user
	cleanupOldAvatars(avatarDir, username)

	// Write new avatar file
	filename := username + ext
	filePath := filepath.Join(avatarDir, filename)
	if err := os.WriteFile(filePath, decoded, 0644); err != nil {
		return "", fmt.Errorf("failed to write avatar file: %w", err)
	}

	return "/uploads/avatars/" + filename, nil
}

func cleanupOldAvatars(dir, username string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, username+".") || strings.HasPrefix(name, username+"_") {
			os.Remove(filepath.Join(dir, name))
		}
	}
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
