package classify

import (
	"strings"
	"testing"

	"github.com/bedri/open-directory-crawler/internal/models"
)

func TestFileEntry(t *testing.T) {
	tests := []struct {
		name string
		size int64
		want models.FileCategory
	}{
		// Video
		{"video.mp4", 0, models.CategoryVideo},
		{"video.MKV", 0, models.CategoryVideo},
		{"video.AVI", 0, models.CategoryVideo},
		// Audio
		{"song.mp3", 0, models.CategoryAudio},
		{"song.FLAC", 0, models.CategoryAudio},
		{"song.wav", 0, models.CategoryAudio},
		// Image
		{"photo.jpg", 0, models.CategoryImage},
		{"photo.JPEG", 0, models.CategoryImage},
		{"photo.png", 0, models.CategoryImage},
		// Document
		{"doc.pdf", 0, models.CategoryDocument},
		{"doc.txt", 0, models.CategoryDocument},
		{"doc.epub", 0, models.CategoryDocument},
		// Archive
		{"archive.zip", 0, models.CategoryArchive},
		{"archive.rar", 0, models.CategoryArchive},
		{"archive.tar.gz", 0, models.CategoryArchive},
		// Code
		{"main.go", 0, models.CategoryCode},
		{"main.py", 0, models.CategoryCode},
		{"index.html", 0, models.CategoryCode},
		{"style.css", 0, models.CategoryCode},
		// Torrent
		{"file.torrent", 0, models.CategoryTorrent},
		// Executable
		{"setup.exe", 0, models.CategoryExecutable},
		// Other
		{"file.xyz", 0, models.CategoryOther},
		{"noext", 0, models.CategoryOther},
		{"", 0, models.CategoryOther},
	}
	for _, tt := range tests {
		got := FileEntry(tt.name, tt.size)
		if got != tt.want {
			t.Errorf("FileEntry(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestExtension(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"file.mp3", "mp3"},
		{"file.MP3", "mp3"},
		{"archive.tar.gz", "gz"},
		{"file", ""},
		{".hidden", "hidden"},
		{"", ""},
		{"path/to/file.pdf", "pdf"},
	}
	for _, tt := range tests {
		got := Extension(tt.name)
		if got != tt.want {
			t.Errorf("Extension(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestIsDirIndex(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"", true},
		{"/", true},
		{"index.html", true},
		{"index.htm", true},
		{"INDEX.HTML", true},
		{"index.HTM", true},
		{"file.html", false},
		{"index", false},
		{"index.php", false},
	}
	for _, tt := range tests {
		got := IsDirIndex(tt.name)
		if got != tt.want {
			t.Errorf("IsDirIndex(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestExtMapCoverage(t *testing.T) {
	knownExts := []string{
		".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".mpg", ".mpeg", ".vob", ".3gp",
		".mp3", ".flac", ".wav", ".aac", ".ogg", ".wma", ".m4a", ".opus", ".ac3", ".ape",
		".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg", ".ico", ".tiff", ".tif", ".raw", ".psd",
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".rtf", ".epub", ".mobi", ".cbr", ".cbz",
		".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz", ".zst", ".iso", ".cab", ".dmg", ".deb", ".rpm",
		".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".h", ".rs", ".rb", ".php", ".html", ".css", ".json", ".xml", ".yaml", ".yml", ".sh", ".bat", ".sql",
		".torrent", ".magnet",
		".exe", ".msi", ".appimage",
	}
	for _, ext := range knownExts {
		if _, ok := extMap[ext]; !ok {
			t.Errorf("extMap missing known extension: %s", ext)
		}
	}
}

func TestExtensionSanitizesGarbage(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"file.mp3", "mp3"},
		{"file.MP3", "mp3"},
		{"archive.tar.gz", "gz"},
		{"file", ""},
		{".hidden", "hidden"},
		{">", ""},
		{"↗", ""},
		{"file.\n\t\t\tgarbage", ""},
		{"file.\t\t\t", ""},
		{"file.a b", ""},
		{"file.abc123", "abc123"},
		{"file." + strings.Repeat("a", 21), ""},
		{"file.a-b_c", "a-b_c"},
	}
	for _, tt := range tests {
		got := Extension(tt.name)
		if got != tt.want {
			t.Errorf("Extension(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}
