package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
)

type Store interface {
	Put(ctx context.Context, key string, value []byte) error
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Delete(ctx context.Context, key string) error
	Close() error
}

type BoltStore struct {
	db     *bbolt.DB
	bucket []byte
}

func NewBoltStore(path, bucket string) (*BoltStore, error) {
	if path == "" {
		return nil, errors.New("Путь к БД не указан.")
	}
	if bucket == "" {
		return nil, errors.New("Имя bucket'а не указано.")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}
	store := &BoltStore{db: db, bucket: []byte(bucket)}
	if err := store.ensureBucket(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *BoltStore) ensureBucket() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(s.bucket)
		return err
	})
}

func (s *BoltStore) Put(ctx context.Context, key string, value []byte) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(s.bucket)
		if bucket == nil {
			return errors.New("bucket не найден.")
		}
		return bucket.Put([]byte(key), value)
	})
}

func (s *BoltStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, false, ctx.Err()
	}
	var out []byte
	if err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(s.bucket)
		if bucket == nil {
			return errors.New("bucket не найден.")
		}
		value := bucket.Get([]byte(key))
		if value == nil {
			return nil
		}
		out = make([]byte, len(value))
		copy(out, value)
		return nil
	}); err != nil {
		return nil, false, err
	}
	if out == nil {
		return nil, false, nil
	}
	return out, true, nil
}

func (s *BoltStore) Delete(ctx context.Context, key string) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(s.bucket)
		if bucket == nil {
			return errors.New("bucket не найден.")
		}
		return bucket.Delete([]byte(key))
	})
}

func (s *BoltStore) Close() error {
	return s.db.Close()
}
