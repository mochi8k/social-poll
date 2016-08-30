package main

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/garyburd/go-oauth/oauth"
	"github.com/joeshaw/envdecode"
)

var conn net.Conn

/*
	新しい接続を開き、コネクションを更新後、コネクションを返却する.
  既に接続されているコネクションが閉じられていない場合は閉じる.
*/
func dial(netw, addr string) (net.Conn, error) {
	if conn != nil {
		conn.Close()
		conn = nil
	}

	// タイムアウトを検知するコネクション.
	netc, err := net.DialTimeout(netw, addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	conn = netc
	return netc, nil
}

var reader io.ReadCloser

/*
	コネクションとReadCloserを閉じる.
*/
func closeConn() {
	if conn != nil {
		conn.Close()
	}
	if reader != nil {
		reader.Close()
	}
}

var (
	creds      *oauth.Credentials
	authClient *oauth.Client
)

/*
	環境変数から認証情報を読み込み、OAuthオブジェクトのセットアップを行う.
	OAuthオブジェクトはリクエストの認証に使う.
*/
func setupTwitterAuth() {
	var ts struct {
		ConsumerKey    string `env:"SP_TWITTER_KEY,required"`
		ConsumerSecret string `env:"SP_TWITTER_SECRET,required"`
		AccessToken    string `env:"SP_TWITTER_ACCESSTOKEN,required"`
		AccessSecret   string `env:"SP_TWITTER_ACCESSSECRET,required"`
	}
	if err := envdecode.Decode(&ts); err != nil {
		log.Fatalln("Twitterの認証情報が環境変数に設定されていません.", err)
	}
	creds = &oauth.Credentials{
		Token:  ts.AccessToken,
		Secret: ts.AccessSecret,
	}
	authClient = &oauth.Client{
		Credentials: oauth.Credentials{
			Token:  ts.ConsumerKey,
			Secret: ts.ConsumerSecret,
		},
	}
}

var authSetupOnce sync.Once

/*
	認証情報を付与し、リクエストを送信する.
	認証情報のセットアップは初回コール時のみ.
*/
func makeRequest(req *http.Request, params url.Values) (*http.Response, error) {

	var httpClient *http.Client

	// do once.
	authSetupOnce.Do(func() {
		setupTwitterAuth()
		httpClient = &http.Client{
			Transport: &http.Transport{
				Dial: dial,
			},
		}
	})
	formEnc := params.Encode()
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Content-Length", strconv.Itoa(len(formEnc)))
	req.Header.Set("Authorization",
		authClient.AuthorizationHeader(creds, "POST", req.URL, params))
	return httpClient.Do(req)
}

type tweet struct {
	Text string
}

/*
	Twitter APIを使い、検索のリクエストを送信する.
	検索結果のツイート内で、全ての選択肢(options)が言及されている場合、投票を行う.
	votes: 送信専用(<-)のチャネル
*/
func readFromTwitter(votes chan<- string) {
	options, err := loadOptions()
	if err != nil {
		log.Println("選択肢の読み込みに失敗:", err)
		return
	}

	// Twitter側のエンドポイントを指すurl.URLオブジェクトを生成.
	u, err := url.Parse("https://stream.twitter.com/1.1/statuses/filter.json")
	if err != nil {
		log.Println("URLの解析に失敗:", err)
		return
	}

	query := make(url.Values)
	query.Set("track", strings.Join(options, ","))
	req, err := http.NewRequest("POST", u.String(), strings.NewReader(query.Encode()))
	if err != nil {
		log.Println("検索のリクエスト作成に失敗:", err)
		return
	}

	resp, err := makeRequest(req, query)
	if err != nil {
		log.Println("検索のリクエストに失敗:", err)
		return
	}

	reader = resp.Body
	decoder := json.NewDecoder(reader)
	for {
		var tweet tweet
		if err := decoder.Decode(&tweet); err != nil {
			break
		}
		for _, option := range options {
			if strings.Contains(strings.ToLower(tweet.Text), strings.ToLower(option)) {
				log.Println("投票:", option)
				votes <- option
			}
		}
	}

}
