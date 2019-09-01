package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
	"path"

	"github.com/gorilla/mux"
	"github.com/jpicht/podcast"
	"github.com/spf13/pflag"
)

var (
	listen      = pflag.StringP("listen", "l", ":8100", "Listen address")
	title       = pflag.StringP("title", "t", "podsrv", "The podcast title")
	description = pflag.StringP("description", "d", "", "The podcast description")

	folder string
)

type (
	file struct {
		name string
		date time.Time
	}
	fileList []file
)

func (f fileList) Len() int {
	return len(f)
}

func (f fileList) Less(i, j int) bool {
	return f[i].date.Before(f[j].date)
}

func (f fileList) Swap(i, j int) {
	x := f[i]
	f[i] = f[j]
	f[j] = x
}

func logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.URL)
		next.ServeHTTP(w, r)
	})
}

func listFiles(folder string) (fileList, string, error) {
	// finding and sorting
	files, err := ioutil.ReadDir(folder)
	if err != nil {
		return nil, "", err
	}

	all := make(fileList, 0, len(files))

	txt := make([]string, 0, len(files))

	for _, item := range files {
		if item.IsDir() {
			continue
		}
		if strings.HasSuffix(item.Name(), ".mp3") {
			all = append(all, file{
				item.Name(),
				item.ModTime(),
			})
		} else if strings.HasSuffix(item.Name(), ".txt") {
			txt = append(txt, item.Name())
		}
	}

	sort.Sort(sort.Reverse(all))

	if len(txt) == 1 {
		desc, err := ioutil.ReadFile(path.Join(folder, txt[0]))
		if err == nil {
			return all, string(desc), nil
		}
	}

	return all, "", nil
}

func handle(w http.ResponseWriter, r *http.Request) {
	files, txt, err := listFiles(folder)
	if err != nil {
		log.Fatal(err)
	}

	if *description != " " {
		txt = *description
	}

	now := time.Now()
	p := podcast.New(
		*title,
		"/index.xml",
		txt,
		now, now,
	)

	// building
	for _, file := range files {
		p.AddItem(podcast.Item{
			Title:       file.name,
			Description: file.name,
			Link:        "http://" + r.Host + "/files/" + file.name,
			PubDate:     file.date,
		})
	}

	// output
	w.Header().Set("Content-Type", "application/xml")
	if err := p.Encode(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	pflag.Parse()
	args := pflag.Args()
	if len(args) > 0 {
		folder = args[0]
	} else {
		folder = "."
	}

	files, txt, err := listFiles(folder)
	if err != nil {
		log.Fatal(err)
		return
	}
	log.Println("Current content:")
	log.Println(txt)
	for _, file := range files {
		log.Println("    ", file.name, file.date)
	}

	r := mux.NewRouter()
	r.HandleFunc("/", handle)
	r.HandleFunc("/index.xml", handle)
	r.PathPrefix("/files/").Handler(http.StripPrefix("/files", http.FileServer(http.Dir(folder))))
	http.Handle("/", logger(r))

	http.ListenAndServe(*listen, nil)
}
