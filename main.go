package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	c := config{
		Host:      os.Getenv("HOSTNAME"),
		Cert:      os.Getenv("CERT"),
		Key:       os.Getenv("KEY"),
		Site:      os.Getenv("SITE"),
		AliasFile: os.Getenv("ALIAS"),
		UseTLS:    true,
	}
	if c.Site == "" {
		log.Fatal("Specify site directory")
	}
	if c.Host == "" {
		c.Host = "localhost:8080"
	}
	if c.Cert == "" && c.Key == "" {
		c.UseTLS = false
	}
	if err := run(c); err != nil {
		log.Fatal(err)
	}
}

type config struct {
	Host      string
	Cert      string
	Key       string
	Site      string
	AliasFile string
	UseTLS    bool
}

func run(c config) error {
	aliases, err := getAliases(c.AliasFile)
	if err != nil {
		if c.AliasFile != "" {
			log.Printf("couldn't load aliases: %s", err)
		}
	}
	fs, err := newFS(c.Site)
	if err != nil {
		return err
	}

	m := http.ServeMux{}
	m.Handle("/", logthis(handle(aliases, fs)))
	srv := &http.Server{
		ReadTimeout:  time.Second,
		WriteTimeout: 2 * time.Second,
		Handler:      &m,
	}

	srv.SetKeepAlivesEnabled(false)
	log.Printf("Start server: %s", c.Host)
	if c.UseTLS {
		srv.Addr = ":443"
		go http.ListenAndServe(":80", http.HandlerFunc(redirect(c.Host)))
		err = srv.ListenAndServeTLS(c.Cert, c.Key)
	} else {
		srv.Addr = ":8080"
		err = srv.ListenAndServe()
	}
	return err
}

func getAliases(filename string) (map[string]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("opening alias file %q %w", filename, err)
	}
	defer f.Close()

	var aliases map[string]string
	if err := json.NewDecoder(f).Decode(&aliases); err != nil {
		return nil, fmt.Errorf("alias json decoding: %w", err)
	}
	return aliases, nil
}

type FS map[string][]byte

func newFS(dir string) (FS, error) {
	cache, err := walk(dir, ".git")
	if err != nil {
		return nil, err
	}
	return FS(cache), nil
}

func walk(dir string, skipDirs ...string) (map[string][]byte, error) {
	cache := make(map[string][]byte)
	err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			for _, skip := range skipDirs {
				if info.Name() == skip {
					return filepath.SkipDir
				}
			}
			// continue we don't do any listing of directories
			return nil
		}
		by, err := ioutil.ReadFile(path)
		if err != nil {
			return fmt.Errorf("couldn't read %q: %s", path, err)
		}
		stripped := strings.TrimPrefix(path, dir)
		cache[stripped] = by
		return nil
	})
	return cache, err
}

func redirect(host string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Host != host {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		target := "https://" + r.Host + r.URL.Path
		if len(r.URL.RawQuery) > 0 {
			target += "?" + r.URL.RawQuery
		}
		log.Printf("redirect to: %s", target)
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	}
}

// ResponseWriter is a wrapper for http.ResponseWriter to get the written http status code
type ResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (r *ResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.statusCode = statusCode
}

func (r *ResponseWriter) StatusCode() int {
	if r.statusCode == 0 {
		return http.StatusOK
	}
	return r.statusCode
}

func logthis(h http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wr := &ResponseWriter{ResponseWriter: w}
		h(wr, r)
		since := time.Since(start)
		log.Println(since, wr.StatusCode(), r.Method, r.URL)
	})
}

// The algorithm for detecting ContentType uses at most sniffLen bytes to make its decision.
const sniffLen = 512

func handle(aliases map[string]string, fs FS) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ext := filepath.Ext(r.URL.Path)
		content, ok := fs[r.URL.Path]
		if !ok {
			alias, ok := aliases[r.URL.Path]
			if ok {
				if len(r.URL.RawQuery) > 0 {
					alias += "?" + r.URL.RawQuery
				}
				log.Printf("redirect to: %s", alias)
				http.Redirect(w, r, alias, http.StatusTemporaryRedirect)
				return
			}
			// Allow for .html files to be addressed without it
			ext = "html"
			content, ok = fs[r.URL.Path+"."+ext]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(http.StatusText(http.StatusNotFound)))
				return
			}
		}
		ctype := mime.TypeByExtension(ext)
		if ctype == "" {
			// read a chunk to decide between utf-8 text and binary
			ctype = http.DetectContentType(content[:sniffLen])
		}
		w.Header().Set("Content-Type", ctype)
		w.Write(content)
		return
	})
}
