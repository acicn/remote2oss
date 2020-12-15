package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	badPathCharacters = regexp.MustCompile(`[^0-9a-zA-Z._/-]`)
)

func sanitizePath(name string) string {
	return strings.ToLower(badPathCharacters.ReplaceAllString(name, "_"))
}

func fileExists(filename string) (ok bool, err error) {
	if _, err = os.Stat(filename); err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return
	}
	ok = true
	return
}

func exit(err *error) {
	if *err != nil {
		log.Println("exited with error:", (*err).Error())
		os.Exit(1)
	}
}

type Options struct {
	Workspace          string `json:"workspace"`
	OSSPublicURL       string `json:"oss_public_url"`
	OSSEndpoint        string `json:"oss_endpoint"`
	OSSAccessKeyID     string `json:"oss_access_key_id"`
	OSSAccessKeySecret string `json:"oss_access_key_secret"`
	OSSBucket          string `json:"oss_bucket"`
}

var (
	optConf     string
	optLocation string
)

func main() {
	var err error
	defer exit(&err)

	var home string
	if home, err = os.UserHomeDir(); err != nil {
		return
	}

	flag.StringVar(&optConf, "c", filepath.Join(home, ".remote2oss.json"), "configuration file")
	flag.StringVar(&optLocation, "l", "", "url to download")
	flag.Parse()

	optLocation = strings.TrimSpace(optLocation)

	if optLocation == "" {
		err = errors.New("missing argument '-l'")
		return
	}

	var buf []byte
	if buf, err = ioutil.ReadFile(optConf); err != nil {
		return
	}

	var opts Options
	if err = json.Unmarshal(buf, &opts); err != nil {
		return
	}

	log.Println("oss go sdk version: ", oss.Version)

	var client *oss.Client
	if client, err = oss.New(opts.OSSEndpoint, opts.OSSAccessKeyID, opts.OSSAccessKeySecret); err != nil {
		return
	}

	log.Println("bucket:", opts.OSSBucket)

	var bucket *oss.Bucket
	if bucket, err = client.Bucket(opts.OSSBucket); err != nil {
		return
	}

	log.Println("url:", optLocation)

	var l *url.URL
	if l, err = url.Parse(optLocation); err != nil {
		return
	}

	filename := filepath.Join(opts.Workspace, sanitizePath(path.Base(l.Path)))
	log.Println("local file:", filename)

	var fe bool
	if fe, err = fileExists(filename); err != nil {
		return
	}

	if !fe {
		defer os.Remove(filename)

		var f *os.File
		if f, err = os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0640); err != nil {
			return
		}
		defer f.Close()

		var res *http.Response
		if res, err = http.Get(optLocation); err != nil {
			return
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			err = fmt.Errorf("bad code: %d", res.StatusCode)
			return
		}

		if _, err = io.Copy(f, res.Body); err != nil {
			return
		}
	}

	key := path.Join(l.Host, sanitizePath(l.Path))
	log.Println("remote file:", strings.TrimSuffix(opts.OSSPublicURL, "/")+"/"+strings.TrimPrefix(key, "/"))
	if err = bucket.PutObjectFromFile(key, filename); err != nil {
		return
	}

	log.Println("done")
}
