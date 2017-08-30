package storage

import (
	"bytes"
	"encoding/gob"
	"math/rand"
	"sync"
	"time"

	"github.com/Ssawa/bolt"
	"github.com/Ssawa/destiny/utils"
	"github.com/satori/go.uuid"
)

// Reuse components to speed up serialization and deserialization
var adageBuffer = new(bytes.Buffer)
var adageEncoder = gob.NewEncoder(adageBuffer)
var adageDecoder = gob.NewDecoder(adageBuffer)
var adageMutex sync.Mutex

// Adage is an entry in the database
type Adage struct {
	ID        uuid.UUID
	Body      string
	Tags      []string
	Author    string
	Source    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// GetAdage gets a random adage from the database
func GetAdage(db *bolt.DB) (*Adage, error) {
	var adage *Adage
	var keys [][]byte

	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(adagesKey)
		if bucket == nil {
			utils.Verbose.Println("Adages bucket does not exist in the database")
			return nil
		}
		c := bucket.Cursor()

		utils.Verbose.Println("Iterating over keys")
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			keys = append(keys, k)
		}

		choice := keys[rand.Intn(len(keys))]

		utils.Verbose.Println("Chose: ", choice)

		var err error
		adage, err = DeserializeAdage(bucket.Get(choice))

		return err
	})

	return adage, err
}

// Insert the adage to the database
func (adage *Adage) Insert(db *bolt.DB) error {
	utils.Verbose.Println("Inserting adage")

	id := uuid.NewV1()
	utils.Verbose.Println("UUID generated: ", id)

	utils.Verbose.Println("Starting transaction")
	return db.Update(func(tx *bolt.Tx) error {
		dataBucket, err := tx.CreateBucketIfNotExists(adagesKey)
		if err != nil {
			return err
		}

		utils.Verbose.Println("Saving to database")
		data, err := adage.Serialize()
		if err != nil {
			return err
		}

		err = dataBucket.Put(id.Bytes(), data)
		if err != nil {
			return err
		}

		// Update our tags indexes
		tagsBucket, err := tx.CreateBucketIfNotExists(tagsKey)
		if err != nil {
			return err
		}

		for _, tag := range adage.Tags {
			utils.Verbose.Println("Saving index for tag", tag)
			index, err := tagsBucket.CreateBucketIfNotExists([]byte(tag))
			if err != nil {
				return err
			}
			err = index.Put(id.Bytes(), []byte{})
		}

		return nil
	})
}

// Serialize converts the structure to a byte array for saving into the database
func (adage *Adage) Serialize() ([]byte, error) {
	adageMutex.Lock()
	adageBuffer.Reset()
	err := adageEncoder.Encode(*adage)
	if err != nil {
		return nil, err
	}
	data := adageBuffer.Bytes()
	adageMutex.Unlock()
	return data, nil
}

// DeserializeAdage converts a byte array into an Adage struct
func DeserializeAdage(data []byte) (*Adage, error) {
	adageMutex.Lock()
	adageBuffer.Reset()
	adageBuffer.Write(data)
	adage := new(Adage)
	err := adageDecoder.Decode(adage)
	adageMutex.Unlock()
	return adage, err
}

// SerializeDirect is another implementation of Serialize that doesn't reuse cached
// components. See adage_test's TestAdageSerializeDeSerialize for an example of
// the time difference.
func (adage *Adage) SerializeDirect() ([]byte, error) {
	buffer := new(bytes.Buffer)
	encoder := gob.NewEncoder(buffer)
	err := encoder.Encode(*adage)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// DeserializeAdageDirect is another implementation of DeserializeAdage that doesn't
// reuse cached components. See adage_test's TestAdageSerializeDeSerialize for an
// example of the time difference.
func DeserializeAdageDirect(data []byte) (*Adage, error) {
	buffer := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buffer)
	adage := new(Adage)
	err := decoder.Decode(adage)
	return adage, err
}
