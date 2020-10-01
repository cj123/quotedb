package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cj123/formulate"
	"github.com/cj123/formulate/decorators"
	"github.com/go-chi/chi"
)

var (
	quotesFolder string
	password     string
)

func init() {
	flag.StringVar(&quotesFolder, "f", "quotes", "where to store the quotes")
	flag.StringVar(&password, "p", "banana", "password")
	flag.Parse()
}

func main() {
	if err := os.MkdirAll(quotesFolder, 0755); err != nil {
		panic(err)
	}

	indexTmpl := template.Must(template.New("index").Parse(indexTemplate))

	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		quotes, err := listQuotes()

		if err != nil {
			http.Error(w, "oh god its broken", http.StatusInternalServerError)
			return
		}

		indexTmpl.Execute(w, map[string]interface{}{
			"Quotes": quotes,
		})
	})

	r.Get("/add-quote", func(w http.ResponseWriter, r *http.Request) {
		form, err := encodeForm(AddQuoteForm{})

		if err != nil {
			http.Error(w, "could not encode form", http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "text/html")
		_, _ = w.Write([]byte(fmt.Sprintf(addQuoteTemplate, form)))
	})

	r.Post("/submit", func(w http.ResponseWriter, r *http.Request) {
		var form AddQuoteForm

		if err := decodeForm(r, &form); err != nil {
			http.Error(w, "bad form", http.StatusInternalServerError)
			return
		}

		if string(form.WhatIsThePassword) != password {
			http.Error(w, "bad password", http.StatusForbidden)
			return
		}

		form.Quote.Time = time.Now()

		if err := form.Quote.Save(); err != nil {
			http.Error(w, "couldn't save quote", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusFound)
	})

	log.Fatal(http.ListenAndServe(":8990", r))
}

func encodeForm(data interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)

	err := formulate.NewEncoder(buf, decorators.BootstrapDecorator{}).Encode(data)

	if err != nil {
		return nil, err
	}

	return buf.Bytes(), err
}

func decodeForm(r *http.Request, data interface{}) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	dec := formulate.NewDecoder(r.Form)

	return dec.Decode(data)
}

func listQuotes() ([]*Quote, error) {
	files, err := ioutil.ReadDir(quotesFolder)

	if err != nil {
		return nil, err
	}

	var quotes []*Quote

	for _, file := range files {
		q, err := readQuote(file.Name())

		if err != nil {
			return nil, err
		}

		quotes = append(quotes, q)
	}

	sort.Slice(quotes, func(i, j int) bool {
		return quotes[i].Time.After(quotes[j].Time)
	})

	return quotes, nil
}

func readQuote(file string) (*Quote, error) {
	f, err := os.Open(filepath.Join(quotesFolder, file))

	if err != nil {
		return nil, err
	}

	defer f.Close()

	var quote Quote

	if err := json.NewDecoder(f).Decode(&quote); err != nil {
		return nil, err
	}

	return &quote, nil
}

type Quote struct {
	Time                     time.Time `show:"-"`
	WhoSaidTheSillyThing     string    `name:"Who said the silly thing?"`
	WhatSillyThingDidTheySay string    `name:"What silly thing did they say?" elem:"textarea"`
}

func (q *Quote) IsImageURL() bool {
	_, err := url.Parse(q.WhatSillyThingDidTheySay)

	return err == nil && strings.Contains(q.WhatSillyThingDidTheySay, "http") &&
		(strings.HasSuffix(q.WhatSillyThingDidTheySay, ".png") || strings.HasSuffix(q.WhatSillyThingDidTheySay, ".jpg") || strings.HasSuffix(q.WhatSillyThingDidTheySay, ".jpeg") || strings.HasSuffix(q.WhatSillyThingDidTheySay, ".gif"))
}

func (q *Quote) HTML() template.HTML {
	if q.IsImageURL() {
		return template.HTML(fmt.Sprintf(`<img src="%s" class="img img-fluid" style="max-height: 400px;">`, q.WhatSillyThingDidTheySay))
	}

	return template.HTML(strings.Replace(q.WhatSillyThingDidTheySay, "\r\n", "<br>", -1))
}

func (q *Quote) Save() error {
	f, err := os.Create(filepath.Join(quotesFolder, fmt.Sprintf("%s.json", q.Time.Format("2006-01-02_15-04-05"))))

	if err != nil {
		return err
	}

	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")

	return enc.Encode(q)
}

type AddQuoteForm struct {
	Quote
	WhatIsThePassword formulate.Password `name:"What is the password?" help:"If you don't know this, then you don't belong here."`
}

const indexTemplate = `
<html lang="en">
<head>
	<title>Quotes</title>
	<link rel="stylesheet" type="text/css" href="https://stackpath.bootstrapcdn.com/bootstrap/4.5.2/css/bootstrap.min.css">
	<link rel="stylesheet" type="text/css" href="https://use.fontawesome.com/releases/v5.8.0/css/all.css">
</head>

<body>
	<div class="container">
		<div class="mt-5">

			<div class="float-left">
				<h2>Quotes</h2>
			</div>

			<div class="float-right">
				<a href="/add-quote" class="btn btn-success">Add a Quote</a>
			</div>

			<div class="clearfix"></div>
			
			{{ range $index, $quote := .Quotes }}
				<div class="card mt-5 mb-5">
					<div class="card-body">
						<div class="row">
						<i class="fas fa-quote-left" style="font-size: 4em; color: #0a84ff; float: left; margin-right: 30px; margin-left: 20px;"></i>
				
						<h3 class="mt-4 d-inline-block">{{ $quote.HTML }}</h3>
						</div>

						<div class="float-right text-muted">
							~ {{ $quote.WhoSaidTheSillyThing }}<br> 
							<small>Submitted on {{ $quote.Time.Format "Mon, 02 Jan 2006 15:04:05 MST" }}</small>
						</div>
					</div>
				</div>
			{{ end }}
		</div>

		<footer class="text-right text-muted mb-5">
			<em>yet another useless project by seejy</em>
		</footer>
	</div>
</body>
</html>
`

const addQuoteTemplate = `
<html lang="en">
<head>
	<title>Add a Quote</title>
	<link rel="stylesheet" type="text/css" href="https://stackpath.bootstrapcdn.com/bootstrap/4.5.2/css/bootstrap.min.css">
</head>

<body>
	<div class="container">
		<div class="mt-5">
			<div class="float-left">
				<h2>Submit a Quote</h2>
			</div>

			<div class="float-right">
				<a href="/" class="btn btn-primary">Go Home</a>
			</div>

			<div class="clearfix"></div>
			
			<form method="POST" action="/submit" class="mt-5">
				%s

				<button type="submit" class="btn btn-success float-right">Submit</button>

			</form>

			<div class="clearfix"></div>
		</div>

		<footer class="text-right text-muted mb-5 mt-5">
			<em>yet another useless project by seejy</em>
		</footer>
	</div>
</body>
</html>
`
