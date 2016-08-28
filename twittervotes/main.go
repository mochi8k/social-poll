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

type poll struct {
	Options []string
}

func loadOptions() ([]string, error) {
	var options []string

	// Find(nil): フィルタリングなし.
	iter := db.DB("ballots").C("polls").Find(nil).Iter()

	var p poll
	for iter.Next(&p) {
		options = append(options, p.Options...)
	}
	iter.Close()
	return options, iter.Err()
}
