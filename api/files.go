package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// FileInfo represents file or directory information
type FileInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Type      string `json:"type"` // "file" or "directory"
	Size      int64  `json:"size"`
	Modified  string `json:"modified"`
	IsHidden  bool   `json:"is_hidden"`
}

// FileContentRequest represents a request to read/write file content
type FileContentRequest struct {
	Content string `json:"content"`
}

// sanitizePath ensures the path is within the working directory
func sanitizePath(baseDir, path string) (string, error) {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}

	targetPath := path
	if path == "" || path == "/" || path == "." {
		targetPath = absBase
	} else {
		targetPath = filepath.Join(absBase, path)
	}

	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", err
	}

	relPath, err := filepath.Rel(absBase, absPath)
	if err != nil {
		return "", err
	}

	// Check if path is outside base directory
	if strings.HasPrefix(relPath, "..") {
		return "", os.ErrPermission
	}

	return absPath, nil
}

// ListFiles returns a list of files in the specified directory
func ListFiles(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")

		sanitized, err := sanitizePath(baseDir, path)
		if err != nil {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		file, err := os.Open(sanitized)
		if err != nil {
			http.Error(w, "Failed to open directory", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		files, err := file.Readdir(-1)
		if err != nil {
			http.Error(w, "Failed to read directory", http.StatusInternalServerError)
			return
		}

		var result []FileInfo
		for _, f := range files {
			fileType := "file"
			if f.IsDir() {
				fileType = "directory"
			}

			relPath, _ := filepath.Rel(baseDir, filepath.Join(sanitized, f.Name()))

			result = append(result, FileInfo{
				Name:     f.Name(),
				Path:     relPath,
				Type:     fileType,
				Size:     f.Size(),
				Modified: f.ModTime().Format("2006-01-02T15:04:05Z"),
				IsHidden: strings.HasPrefix(f.Name(), "."),
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// ReadFile reads the content of a file
func ReadFile(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		if path == "" {
			http.Error(w, "Path is required", http.StatusBadRequest)
			return
		}

		sanitized, err := sanitizePath(baseDir, path)
		if err != nil {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		info, err := os.Stat(sanitized)
		if err != nil {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}

		if info.IsDir() {
			http.Error(w, "Cannot read directory as file", http.StatusBadRequest)
			return
		}

		content, err := os.ReadFile(sanitized)
		if err != nil {
			http.Error(w, "Failed to read file", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(content)
	}
}

// WriteFile writes content to a file
func WriteFile(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		path := r.URL.Query().Get("path")
		if path == "" {
			http.Error(w, "Path is required", http.StatusBadRequest)
			return
		}

		var req FileContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		sanitized, err := sanitizePath(baseDir, path)
		if err != nil {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		// Ensure parent directory exists
		dir := filepath.Dir(sanitized)
		if err := os.MkdirAll(dir, 0755); err != nil {
			http.Error(w, "Failed to create directory", http.StatusInternalServerError)
			return
		}

		if err := os.WriteFile(sanitized, []byte(req.Content), 0644); err != nil {
			http.Error(w, "Failed to write file", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "success",
			"path":   path,
		})
	}
}

// DeleteFile deletes a file or directory
func DeleteFile(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		if path == "" {
			http.Error(w, "Path is required", http.StatusBadRequest)
			return
		}

		sanitized, err := sanitizePath(baseDir, path)
		if err != nil {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		info, err := os.Stat(sanitized)
		if err != nil {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}

		if info.IsDir() {
			if err := os.RemoveAll(sanitized); err != nil {
				http.Error(w, "Failed to delete directory", http.StatusInternalServerError)
				return
			}
		} else {
			if err := os.Remove(sanitized); err != nil {
				http.Error(w, "Failed to delete file", http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "success",
			"path":   path,
		})
	}
}

// CreateDirectory creates a new directory
func CreateDirectory(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		path := r.URL.Query().Get("path")
		if path == "" {
			http.Error(w, "Path is required", http.StatusBadRequest)
			return
		}

		sanitized, err := sanitizePath(baseDir, path)
		if err != nil {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		if err := os.MkdirAll(sanitized, 0755); err != nil {
			http.Error(w, "Failed to create directory", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "success",
			"path":   path,
		})
	}
}
