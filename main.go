package main

import (
	"crypto/rand"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
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
	"github.com/gorilla/csrf"
	"mvdan.cc/xurls"
)

var (
	quotesFolder string
	password     string
)

func init() {
	flag.StringVar(&quotesFolder, "f", "quotes", "where to store the quotes")
	flag.StringVar(&password, "p", "password", "password")
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
			http.Error(w, "couldn't list quotes", http.StatusInternalServerError)
			return
		}

		_ = indexTmpl.Execute(w, map[string]interface{}{
			"Quotes": quotes,
		})
	})

	r.HandleFunc("/add-quote", func(w http.ResponseWriter, r *http.Request) {
		var quoteForm AddQuoteForm

		encodedForm, save, err := formulate.Formulate(r, &quoteForm, buildEncoder, buildDecoder)

		if err == nil && save {
			quoteForm.Quote.Time = time.Now()

			if err := quoteForm.Quote.Save(); err != nil {
				http.Error(w, "couldn't save quote", http.StatusInternalServerError)
				return
			}

			http.Redirect(w, r, "/", http.StatusFound)
		} else if err != nil {
			http.Error(w, "bad form", http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "text/html")
		_, _ = w.Write([]byte(fmt.Sprintf(addQuoteTemplate, encodedForm)))
	})

	r.Get("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain")
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /"))
	})

	b := make([]byte, 32)

	_, err := rand.Read(b)

	if err != nil {
		panic(err)
	}

	log.Fatal(http.ListenAndServe(":8990", csrf.Protect(b)(r)))
}

func buildEncoder(r *http.Request, w io.Writer) *formulate.HTMLEncoder {
	enc := formulate.NewEncoder(w, r, decorators.BootstrapDecorator{})
	enc.SetCSRFProtection(true)
	enc.SetFormat(false)

	return enc
}

func buildDecoder(r *http.Request, form url.Values) *formulate.HTTPDecoder {
	dec := formulate.NewDecoder(form)
	dec.SetValueOnValidationError(true)
	dec.AddValidators(passwordValidator{})

	return dec
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

	q.WhatSillyThingDidTheySay = xurls.Relaxed.ReplaceAllString(q.WhatSillyThingDidTheySay, `<a href="$1">$1</a>`)

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
	enc.SetEscapeHTML(false)
	return enc.Encode(q)
}

type AddQuoteForm struct {
	Quote
	WhatIsThePassword formulate.Password `name:"What is the password?" help:"If you don't know this, then you don't belong here." validators:"password"`
}

var (
	//go:embed templates/index.html
	indexTemplate string

	//go:embed templates/add-quote.html
	addQuoteTemplate string
)

type passwordValidator struct{}

func (p passwordValidator) Validate(val interface{}) (ok bool, message string) {
	switch a := val.(type) {
	case string:
		if a == password {
			return true, ""
		}

		return false, "The password is incorrect."
	default:
		return false, ""
	}
}

func (p passwordValidator) TagName() string {
	return "password"
}
