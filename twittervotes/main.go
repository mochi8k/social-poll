package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	nsq "github.com/bitly/go-nsq"

	mgo "gopkg.in/mgo.v2"
)

var db *mgo.Session

/*
	MongoDBインスタンスへの接続を行う.
		returns:
			error: エラーインスタンス
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
		returns:
			[]string: 投票の選択肢.
			error: エラーインスタンス
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
	NSQへ引数で指定した投票内容をパブリッシュする.
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

func main() {
	var stoplock sync.Mutex

	isStop := false
	stopChan := make(chan struct{}, 1)
	signalChan := make(chan os.Signal, 1)

	go func() {

		// チャネルからの読み込み(SIGINT, SIGTERMのみ).
		<-signalChan

		stoplock.Lock()
		isStop = true
		stoplock.Unlock()

		log.Println("停止します...")

		stopChan <- struct{}{}
		closeConn()
	}()

	// 誰かがプログラムを終了させようとした時にsignalChanにシグナルを送信する.
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	if err := dialdb(); err != nil {
		log.Fatalln("MongoDBへのダイヤルに失敗しました:", err)
	}
	defer closedb()

	// 投票結果のためのチャネル
	votes := make(chan string)

	publisherStoppedChan := publishVotes(votes)
	twitterStoppedChan := startTwitterStream(stopChan, votes)

	// 1分間の待機とcloseConnによる切断を繰り返す.
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			closeConn()

			// isStopへのアクセスの競合を回避.
			stoplock.Lock()
			if isStop {
				stoplock.Unlock()
				break
			}
			stoplock.Unlock()
		}
	}()
	<-twitterStoppedChan
	close(votes)
	<-publisherStoppedChan
}
