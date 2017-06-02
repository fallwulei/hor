package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"encoding/json"

	"github.com/boltdb/bolt"
)

type object struct {
	X    int    `json:"x"`
	Y    int    `json:"y"`
	W    int    `json:"w"`
	H    int    `json:"h"`
	Path string `json:"path"`
}

type objects struct {
	Objs []*object `json:"objects"`
}

type image struct {
	ID         uint64    `json:"id"`
	Path       string    `json:"path"`
	Identified int       `json:"identified"`
	Reviewed   int       `json:"reviewed"`
	Objects    []*object `json:"objects"`
}

type store struct {
	db          *bolt.DB
	path        string
	initialized bool
}

var dbFilename = "images_hr.db"
var imgBucketName = []byte("image")

func newStore(path string) (*store, error) {
	dbFileExist := false
	if _, err := os.Stat(dbFilename); err == nil {
		dbFileExist = true
	}
	db, err := bolt.Open(dbFilename, 0600, nil)
	if err != nil {
		return nil, err
	}
	s := &store{
		db,
		path,
		dbFileExist,
	}
	if !dbFileExist {
		log.Printf("initializing db...\n")
		if err := s.init(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *store) init() error {
	imgC := make(chan string)
	err := s.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket(imgBucketName)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Search path for images
	go func() {
		filepath.Walk(s.path, func(path string, f os.FileInfo, err error) error {
			if !f.IsDir() && strings.HasPrefix(f.Name(), "P_") {
				log.Printf("Find a image: %s\n", f.Name())
				imgC <- path
			}
			return nil
		})
		close(imgC)
	}()
	// Write images data into db
	go func() {
		for {
			path, ok := <-imgC
			if !ok {
				break
			}
			img := &image{Path: path}
			if err := s.CreateImage(img); err != nil {
				log.Printf("fail to create image %s, %v\n", path, err)
			} else {
				log.Printf("save %d: %s", img.ID, img.Path)
			}
		}
		s.initialized = true
	}()
	return nil
}

func (s *store) Rebuild() error {
	if err := s.Close(); err != nil {
		return err
	}
	if err := os.Remove(dbFilename); err != nil {
		return err
	}
	s1, err := newStore(s.path)
	if err != nil {
		return err
	}
	s.db = s1.db
	return nil
}

func (s *store) Close() error {
	return s.db.Close()
}

func (s *store) CreateImage(img *image) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(imgBucketName)
		s, err := b.NextSequence()
		if err != nil {
			return err
		}
		img.ID = s
		buf, _ := json.Marshal(img)
		b.Put(itob(s), buf)
		return nil
	})
}

func (s *store) UpdateImage(img *image) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(imgBucketName)
		buf, _ := json.Marshal(img)
		b.Put(itob(img.ID), buf)
		return nil
	})
}

func (s *store) GetImage(id uint64) *image {
	img := &image{}
	s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(imgBucketName)
		v := b.Get(itob(id))
		return json.Unmarshal(v, img)
	})
	return img
}
