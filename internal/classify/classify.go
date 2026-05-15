package classify

import (
	"path/filepath"
	"strings"
	"unicode"

	"github.com/bedri/open-directory-crawler/internal/models"
)

var extMap = map[string]models.FileCategory{
	// Video
	".mp4":  models.CategoryVideo,
	".mkv":  models.CategoryVideo,
	".avi":  models.CategoryVideo,
	".mov":  models.CategoryVideo,
	".wmv":  models.CategoryVideo,
	".flv":  models.CategoryVideo,
	".webm": models.CategoryVideo,
	".m4v":  models.CategoryVideo,
	".mpg":  models.CategoryVideo,
	".mpeg": models.CategoryVideo,
	".vob":  models.CategoryVideo,
	".3gp":  models.CategoryVideo,

	// Audio
	".mp3":  models.CategoryAudio,
	".flac": models.CategoryAudio,
	".wav":  models.CategoryAudio,
	".aac":  models.CategoryAudio,
	".ogg":  models.CategoryAudio,
	".wma":  models.CategoryAudio,
	".m4a":  models.CategoryAudio,
	".opus": models.CategoryAudio,
	".ac3":  models.CategoryAudio,
	".ape":  models.CategoryAudio,

	// Image
	".jpg":  models.CategoryImage,
	".jpeg": models.CategoryImage,
	".png":  models.CategoryImage,
	".gif":  models.CategoryImage,
	".bmp":  models.CategoryImage,
	".webp": models.CategoryImage,
	".svg":  models.CategoryImage,
	".ico":  models.CategoryImage,
	".tiff": models.CategoryImage,
	".tif":  models.CategoryImage,
	".raw":  models.CategoryImage,
	".psd":  models.CategoryImage,

	// Document
	".pdf":  models.CategoryDocument,
	".doc":  models.CategoryDocument,
	".docx": models.CategoryDocument,
	".xls":  models.CategoryDocument,
	".xlsx": models.CategoryDocument,
	".ppt":  models.CategoryDocument,
	".pptx": models.CategoryDocument,
	".txt":  models.CategoryDocument,
	".rtf":  models.CategoryDocument,
	".epub": models.CategoryDocument,
	".mobi": models.CategoryDocument,
	".cbr":  models.CategoryDocument,
	".cbz":  models.CategoryDocument,

	// Archive
	".zip":    models.CategoryArchive,
	".rar":    models.CategoryArchive,
	".7z":     models.CategoryArchive,
	".tar":    models.CategoryArchive,
	".gz":     models.CategoryArchive,
	".bz2":    models.CategoryArchive,
	".xz":     models.CategoryArchive,
	".zst":    models.CategoryArchive,
	".iso":    models.CategoryArchive,
	".cab":    models.CategoryArchive,
	".dmg":    models.CategoryArchive,
	".deb":    models.CategoryArchive,
	".rpm":    models.CategoryArchive,

	// Code
	".go":    models.CategoryCode,
	".py":    models.CategoryCode,
	".js":    models.CategoryCode,
	".ts":    models.CategoryCode,
	".java":  models.CategoryCode,
	".c":     models.CategoryCode,
	".cpp":   models.CategoryCode,
	".h":     models.CategoryCode,
	".rs":    models.CategoryCode,
	".rb":    models.CategoryCode,
	".php":   models.CategoryCode,
	".html":  models.CategoryCode,
	".css":   models.CategoryCode,
	".json":  models.CategoryCode,
	".xml":   models.CategoryCode,
	".yaml":  models.CategoryCode,
	".yml":   models.CategoryCode,
	".sh":    models.CategoryCode,
	".bat":   models.CategoryCode,
	".sql":   models.CategoryCode,

	// Torrent
	".torrent": models.CategoryTorrent,
	".magnet":  models.CategoryTorrent,

	// Executable
	".exe":  models.CategoryExecutable,
	".msi":  models.CategoryExecutable,
	".appimage": models.CategoryExecutable,
}

func FileEntry(name string, size int64) models.FileCategory {
	ext := strings.ToLower(filepath.Ext(name))
	if cat, ok := extMap[ext]; ok {
		return cat
	}
	return models.CategoryOther
}

func Extension(name string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	for _, r := range ext {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' && r != '.' {
			return ""
		}
	}
	if len(ext) > 20 {
		return ""
	}
	return ext
}

func IsDirIndex(name string) bool {
	name = strings.ToLower(name)
	return name == "" || name == "/" || name == "index.html" || name == "index.htm"
}
