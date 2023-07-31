package db

import (
	"fmt"
	"log"

	bolt "go.etcd.io/bbolt"
)

const (
	bucket = "patches"
)

type BoltDB struct {
	db *bolt.DB
}

const (
	Format       = "PATCH-ID-%s-%s"
	IsCompleted  = "IS_COMPLETED"
	HasError     = "HAS_ERROR"
	Processed    = "PROCESSED"
	ErrorMessage = "ERROR_MESSAGE"
)

func NewBoltDB() *BoltDB {
	db, err := bolt.Open("my.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}

	return &BoltDB{db}
}

func (b *BoltDB) Path() string {
	return b.db.Path()
}

func (b *BoltDB) Close() {
	b.db.Close()
}

func (b *BoltDB) Set(key string, val []byte) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bu, err := tx.CreateBucketIfNotExists([]byte(bucket))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}

		if err := bu.Put([]byte(key), val); err != nil {
			return fmt.Errorf("put in patches : %s", err)
		}

		return nil
	})
}

func (b *BoltDB) Get(key string) []byte {
	var value []byte

	b.db.View(func(tx *bolt.Tx) error {
		bu := tx.Bucket([]byte(bucket))

		value = bu.Get([]byte(key))
		return nil
	})

	return value
}