package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/garyburd/go-oauth/oauth"
	"github.com/joeshaw/envdecode"
)

var conn net.Conn

/*
	新しい接続を開き、コネクションを更新する.
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
