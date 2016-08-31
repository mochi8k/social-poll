package main

import (
	"log"

	nsq "github.com/bitly/go-nsq"

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

/*
	NSQへパブリッシュする.
		params:
			votes: 投票内容が送信されるチャネル(受信専用)
		returns:
			<-chan struct{}: goroutineが実行中かどうかを判断するためのチャネル.
*/
func publishVotes(votes <-chan string) <-chan struct{} {
	stopchan := make(chan struct{}, 1)

	pub, _ := nsq.NewProducer("localhost:4150", nsq.NewConfig())

	go func() {

		// votesチャネルを継続的にチェックする.
		// チャネルが空の場合、値が何か差送信されるまで実行はブロックされる.
		// チャネルが閉じられた場合、ループが終了する.
		for vote := range votes {
			// 投票内容をパブリッシュ
			pub.Publish("votes", []byte(vote))
			log.Println(vote)
		}
		log.Println("Publisher: 停止中です")
		pub.Stop()
		log.Println("Publisher: 停止しました")
		stopchan <- struct{}{}
	}()

	return stopchan
}
