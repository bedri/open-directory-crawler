package storage

import (
	"encoding/json"
	"fmt"

	"github.com/bedri/open-directory-crawler/internal/models"
	"github.com/dgraph-io/badger/v4"
)

type Store struct {
	db *badger.DB
}

func New(path string) (*Store, error) {
	opts := badger.DefaultOptions(path).
		WithLogger(nil).
		WithMemTableSize(64 << 20)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("badger open: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func dirKey(id string) []byte         { return []byte("dir:" + id) }
func fileKey(dirID, path string) []byte { return []byte("file:" + dirID + ":" + path) }
func extIdxKey(ext, ref string) []byte { return []byte("idx:ext:" + ext + ":" + ref) }
func catIdxKey(cat models.FileCategory, ref string) []byte { return []byte("idx:cat:" + string(cat) + ":" + ref) }
func extIdxPrefix(ext string) []byte { return []byte("idx:ext:" + ext + ":") }
func catIdxPrefix(cat models.FileCategory) []byte { return []byte("idx:cat:" + string(cat) + ":") }

func (s *Store) SaveDirectory(d *models.Directory) error {
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(dirKey(d.ID), data)
	})
}

func (s *Store) GetDirectory(id string) (*models.Directory, error) {
	var d models.Directory
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(dirKey(id))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &d)
		})
	})
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *Store) ListDirectories() ([]*models.Directory, error) {
	var dirs []*models.Directory
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte("dir:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			var d models.Directory
			if err := it.Item().Value(func(val []byte) error {
				return json.Unmarshal(val, &d)
			}); err != nil {
				return err
			}
			dirs = append(dirs, &d)
		}
		return nil
	})
	return dirs, err
}

func (s *Store) SaveFileEntry(f *models.FileEntry) error {
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	ref := f.DirectoryID + ":" + f.ID
	return s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(fileKey(f.DirectoryID, f.ID), data); err != nil {
			return err
		}
		if err := txn.Set(extIdxKey(f.Ext, ref), []byte(ref)); err != nil {
			return err
		}
		return txn.Set(catIdxKey(f.Category, ref), []byte(ref))
	})
}

func (s *Store) GetFilesByDir(dirID string) ([]*models.FileEntry, error) {
	var files []*models.FileEntry
	prefix := []byte("file:" + dirID + ":")
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			var f models.FileEntry
			if err := it.Item().Value(func(val []byte) error {
				return json.Unmarshal(val, &f)
			}); err != nil {
				return err
			}
			files = append(files, &f)
		}
		return nil
	})
	return files, err
}

func (s *Store) GetFilesByExt(ext string) ([]*models.FileEntry, error) {
	var refs []string
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := extIdxPrefix(ext)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			if err := it.Item().Value(func(val []byte) error {
				refs = append(refs, string(val))
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.resolveFileRefs(refs)
}

func (s *Store) GetFilesByCategory(cat models.FileCategory) ([]*models.FileEntry, error) {
	var refs []string
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := catIdxPrefix(cat)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			if err := it.Item().Value(func(val []byte) error {
				refs = append(refs, string(val))
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.resolveFileRefs(refs)
}

func (s *Store) resolveFileRefs(refs []string) ([]*models.FileEntry, error) {
	var files []*models.FileEntry
	err := s.db.View(func(txn *badger.Txn) error {
		for _, ref := range refs {
			var f models.FileEntry
			item, err := txn.Get(fileFileKeyFromRef(ref))
			if err != nil {
				continue
			}
			if err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &f)
			}); err != nil {
				continue
			}
			files = append(files, &f)
		}
		return nil
	})
	return files, err
}

func fileFileKeyFromRef(ref string) []byte {
	return []byte("file:" + ref)
}

func (s *Store) GetStats() (*models.Stats, error) {
	st := &models.Stats{
		CategoryCounts: make(map[models.FileCategory]int64),
		ExtCounts:      make(map[string]int),
	}

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte("dir:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			var d models.Directory
			if err := it.Item().Value(func(val []byte) error {
				return json.Unmarshal(val, &d)
			}); err != nil {
				return err
			}
			st.TotalDirectories++
		}

		prefix2 := []byte("file:")
		seen := make(map[string]bool)
		for it.Seek(prefix2); it.ValidForPrefix(prefix2); it.Next() {
			var f models.FileEntry
			if err := it.Item().Value(func(val []byte) error {
				return json.Unmarshal(val, &f)
			}); err != nil {
				continue
			}
			if seen[f.ID] {
				continue
			}
			seen[f.ID] = true
			st.TotalFiles++
			st.TotalSize += f.Size
			st.CategoryCounts[f.Category]++
			st.ExtCounts[f.Ext]++
		}
		return nil
	})
	return st, err
}

func (s *Store) DeleteDirectory(id string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		prefix := []byte("file:" + id + ":")
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			if err := txn.Delete(it.Item().Key()); err != nil {
				return err
			}
		}
		return txn.Delete(dirKey(id))
	})
}

func (s *Store) RunGC() error {
	return s.db.RunValueLogGC(0.5)
}
