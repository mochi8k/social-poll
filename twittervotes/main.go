package main

import (
	"log"

	mgo "gopkg.in/mgo.v2"
)

var db *mgo.Session

/*
	MongoDBインスタンスへの接続を行う.
*/
func dialdb() error {
	log.Println("Dialling to MongoDB: localhost")

	var err error
	db, err = mgo.Dial("localhost")

	return err
}

/*
	データベースの接続を閉じる.
*/
func closedb() {
	db.Close()
	log.Println("DB connection was closed")
}

type poll struct {
	Options []string
}

/*
	投票を表すオブジェクトを読み込み、投票の選択肢をすべて取り出して返却する.
*/
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
