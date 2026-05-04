package server

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const maxAvatarSize = 2 * 1024 * 1024 // 2MB

func saveAvatarFile(uploadDir, username, avatarData string) (string, error) {
	// Already a URL path, return as-is
	if strings.HasPrefix(avatarData, "/") {
		return avatarData, nil
	}

	// Not a data URI (emoji or text), return as-is
	if !strings.HasPrefix(avatarData, "data:") {
		return avatarData, nil
	}

	mimePart, base64Data, ok := strings.Cut(avatarData, ",")
	if !ok {
		return "", fmt.Errorf("invalid data URI")
	}

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

	// Validate size
	if len(decoded) > maxAvatarSize {
		return "", fmt.Errorf("avatar too large: %d bytes max", maxAvatarSize)
	}

	// Create upload directory
	avatarDir := filepath.Join(uploadDir, "avatars")
	if err := os.MkdirAll(avatarDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create avatar directory: %w", err)
	}

	// Remove old avatar files for this user
	cleanupOldAvatars(avatarDir, username)

	// Write new avatar file
	safeName := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '.' {
			return '_'
		}
		return r
	}, username)
	filename := safeName + ext
	filePath := filepath.Join(avatarDir, filename)
	if err := os.WriteFile(filePath, decoded, 0644); err != nil {
		return "", fmt.Errorf("failed to write avatar file: %w", err)
	}

	return "/uploads/avatars/" + filename, nil
}

func cleanupOldAvatars(dir, username string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		slog.Warn("failed to read avatar dir for cleanup", "dir", dir, "error", err)
		return
	}
	// Apply same sanitization as saveAvatarFile so dots in usernames match
	safePrefix := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '.' {
			return '_'
		}
		return r
	}, username)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, safePrefix+".") || strings.HasPrefix(name, safePrefix+"_") {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				slog.Warn("failed to remove old avatar", "file", name, "error", err)
			}
		}
	}
}
