package main

import (
	"log"

	mgo "gopkg.in/mgo.v2"
)

var db *mgo.Session

func dialdb() error {
	log.Println("Dialling to MongoDB: localhost")

	var err error
	db, err = mgo.Dial("localhost")

	return err
}

func closedb() {
  db.Close()
  log.Println("DB connection was closed")
}
