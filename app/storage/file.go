package storage

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/sablierapp/sablier/config"
)

type Storage interface {
	Reader() (io.ReadCloser, error)
	Writer() (io.WriteCloser, error)
}

type FileStorage struct {
	file string
	l    *slog.Logger
}

func NewFileStorage(config config.Storage, logger *slog.Logger) (Storage, error) {
	logger = logger.With(slog.String("file", config.File))
	storage := &FileStorage{
		file: config.File,
		l:    logger,
	}

	file, err := os.OpenFile(config.File, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return nil, fmt.Errorf("unable to open file: %w", err)
	}
	defer file.Close()

	stats, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("unable to read file info: %w", err)
	}

	// Initialize file to an empty JSON3
	if stats.Size() == 0 {
		_, err := file.WriteString("{}")
		if err != nil {
			return nil, fmt.Errorf("unable to initialize file to valid json: %w", err)
		}
	}

	logger.Info("storage successfully initialized")

	return storage, nil
}

func (fs *FileStorage) Reader() (io.ReadCloser, error) {
	return os.OpenFile(fs.file, os.O_RDWR|os.O_CREATE, 0755)
}

func (fs *FileStorage) Writer() (io.WriteCloser, error) {
	return os.OpenFile(fs.file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
}
