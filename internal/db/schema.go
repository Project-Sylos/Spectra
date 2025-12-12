package db

import (
	"fmt"

	"go.etcd.io/bbolt"
)

// Bucket names for BoltDB storage
const (
	bucketNodes           = "nodes"
	bucketIndexParentID   = "index_parent_id"
	bucketIndexPath       = "index_path"
	bucketIndexParentPath = "index_parent_path"
	bucketStats           = "stats"
)

// InitializeBuckets creates all required buckets in the BoltDB database
// This replaces the SQL table creation logic
func InitializeBuckets(db *bbolt.DB) error {
	return db.Update(func(tx *bbolt.Tx) error {
		// Create main nodes bucket
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketNodes)); err != nil {
			return fmt.Errorf("failed to create nodes bucket: %w", err)
		}

		// Create index buckets
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketIndexParentID)); err != nil {
			return fmt.Errorf("failed to create index_parent_id bucket: %w", err)
		}

		if _, err := tx.CreateBucketIfNotExists([]byte(bucketIndexPath)); err != nil {
			return fmt.Errorf("failed to create index_path bucket: %w", err)
		}

		if _, err := tx.CreateBucketIfNotExists([]byte(bucketIndexParentPath)); err != nil {
			return fmt.Errorf("failed to create index_parent_path bucket: %w", err)
		}

		// Create stats bucket
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketStats)); err != nil {
			return fmt.Errorf("failed to create stats bucket: %w", err)
		}

		return nil
	})
}

// VerifyBucketsExist checks if all required buckets exist in the database
// This replaces the SQL table existence checks
func VerifyBucketsExist(db *bbolt.DB) error {
	return db.View(func(tx *bbolt.Tx) error {
		requiredBuckets := []string{
			bucketNodes,
			bucketIndexParentID,
			bucketIndexPath,
			bucketIndexParentPath,
			bucketStats,
		}

		for _, bucketName := range requiredBuckets {
			bucket := tx.Bucket([]byte(bucketName))
			if bucket == nil {
				return fmt.Errorf("required bucket %s does not exist", bucketName)
			}
		}

		return nil
	})
}
