package main

import (
	"database/sql" //package provides a generic api that allows for interacting with the databases in a vendor-neutral way
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq" //This is a driver; this is the go package for the sql database driver; third-party package

	"readinglist/internal/data"
)

const version = "2.0.0"

type config struct {
	port int
	env  string
	dsn  string // short for data name service; aka a data connection string; this will be passed in so we can connect to the database
}

type application struct {
	config config
	logger *log.Logger
	models data.Models
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	var cfg config

	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	cfg.dsn = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPassword, dbHost, dbPort, dbName)

	flag.IntVar(&cfg.port, "port", 3001, "API server port")
	flag.StringVar(&cfg.env, "env", "dev", "Environment (dev|stage|prod)")
	flag.Parse()

	fmt.Println("hello")

	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime)

	//below opens the database connection
	db, err := sql.Open("postgres", cfg.dsn)
	if err != nil {
		logger.Fatal(err)
	}

	err = db.Ping() //this tests the connection
	if err != nil {
		logger.Fatal(err)
	}

	defer db.Close() //this closes the connection

	logger.Printf("database connection pool established")

	app := &application{
		config: cfg,
		logger: logger,
		models: data.NewModels(db),
	}

	addr := fmt.Sprintf(":%d", cfg.port)

	srv := &http.Server{
		Addr:         addr,
		Handler:      app.route(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	logger.Printf("starting %s server on %s", cfg.env, addr)
	err = srv.ListenAndServe()
	logger.Fatal(err)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (app *application) route() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/healthcheck", app.healthcheck) // this is an route
	// Endpoints are functions available through the API
	// A route is the name you use to access endpoints, used in the URL

	mux.HandleFunc("/v1/books", app.getCreateBooksHandler) // Gets all books with the GET method, Creates new book with the POST method
	//1st arg is the route; 2nd arg is the handler function (endpoint)

	mux.HandleFunc("/v1/books/", app.getUpdateDeleteBooksHandler) // Handles queries related to individual books

	return corsMiddleware(mux) //This returns the mux and all the handlers associated with it
}

// app method handling healthcheck endpoint
func (app *application) healthcheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	//the encoding/marshalling for the healthcheck endpoint will be done differently from the others
	//it's not going to use a struct to convert the json to and from the messages, it's going to use native types
	//it's going to assume based on the data type of the go object itself what type of json values should be marshalled into the response

	//using the data variable is expected
	data := map[string]string{
		"status":      "available",
		"environment": app.config.env,
		"version":     version,
	}
	//below turns the data map from above into json
	js, err := json.Marshal(data)

	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return // this exits, stopping the rest of the code from running
	}
	//This formats the json some
	js = append(js, '\n')

	//Below sets the http headers
	w.Header().Set("Content-Type", "application/json")

	//Below write the http response - we pass in the json object and that is what will be written in the response
	w.Write(js)
}

// This is a Handler - an app method handling getting and creating new books within the total list of books

func (app *application) getCreateBooksHandler(w http.ResponseWriter, r *http.Request) {
	//the if statement validates that the request at this endpoint is only either GET or POST

	//if the endpoint /v1/books is used with get, it does the following
	if r.Method == http.MethodGet {
		//The variable book defines a slice of the data type called Book
		books, err := app.models.Books.GetAll()
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		//The code below calls the helper.go function to format, marshall, and write the json
		//the envelope that is wrapping the books variable is naming that collection of data books and then returning the data of the books variable
		if err := app.writeJSON(w, http.StatusOK, envelope{"books": books}, nil); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

	}
	//if the endpoint /v1/books is used with post, it does the following
	if r.Method == http.MethodPost {

		//below are the pieces of information we expect that will then be unmarshalled into a go object

		var input struct {
			Title     string   `json:"title"`
			Author    string   `json:"author"`
			Published int      `json:"published"`
			Pages     int      `json:"pages"`
			Genres    []string `json:"genres"`
			Rating    float32  `json:"rating"`
			ISBN      string   `json:"isbn"`
		}

		err := app.readJSON(w, r, &input)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		book := &data.Book{
			Title:     input.Title,
			Author:    input.Author,
			Published: input.Published,
			Pages:     input.Pages,
			Genres:    input.Genres,
			Rating:    input.Rating,
			ISBN:      input.ISBN,
		}

		err = app.models.Books.Insert(book)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		//this makes the application aware of the new location for the new book
		headers := make(http.Header)                                 //this makes the new header for the http response
		headers.Set("Location", fmt.Sprintf("v1/books/%d", book.ID)) //this sets the location of the book to the value of the the books/ api with the new book's id appended to it

		//This writes the JSON response with a 201 Created status code and the Location header set
		err = app.writeJSON(w, http.StatusCreated, envelope{"book": book}, headers)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

}

// This is another Handler - an app method handling the get, update, deleting specific books
// Below is a request multiplexer (aka a request router). It routes incoming requests to a handler using a set of rules
func (app *application) getUpdateDeleteBooksHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		app.getBook(w, r)

	case http.MethodPut:
		app.updateBook(w, r)

	case http.MethodDelete:
		app.deleteBook(w, r)

	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

//Below is the definition of each specific case above

// each of the methods below need to have a way to get the id of the book in question from the URL
// getting a specific book
func (app *application) getBook(w http.ResponseWriter, r *http.Request) {
	//below is where we get access the book id from the url
	id := r.URL.Path[len("/v1/books/"):]
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	//this will be removed when this application si connection to a database
	//this is using the struct from the internal/data package
	book, err := app.models.Books.Get(idInt)
	if err != nil {
		switch {
		case errors.Is(err, errors.New("record not found")):
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		default:
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	//The code below calls the helper.go function to format, marshall, and write the json
	//the envelope that is wrapping the book variable is naming that collection of data book and then returning the data of the book variable
	if err := app.writeJSON(w, http.StatusOK, envelope{"book": book}, nil); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

}

func (app *application) updateBook(w http.ResponseWriter, r *http.Request) {
	//below is where we get access the book id from the url
	id := r.URL.Path[len("/v1/books/"):]
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		http.Error(w, "Bad Request2", http.StatusBadRequest)
		return
	}

	book, err := app.models.Books.Get(idInt) //this calls the database to get the specific book record with the id from the url
	if err != nil {
		switch {
		case errors.Is(err, errors.New("record not found")):
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		default:
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	//the struct below defines the way that we want to unmarshall the json
	//we are using pointers because we want to modify the existing struct instead of creating a new one

	var input struct {
		Title     *string  `json:"title"`
		Author    *string  `json:"author"`
		Published *int     `json:"published"`
		Pages     *int     `json:"pages"`
		Genres    []string `json:"genres"` //not sure why this one isn't a pointer?
		Rating    *float32 `json:"rating"`
		ISBN      *string  `json:"isbn"`
	}

	//uses the helper function to unmarshall the json into a go object
	err = app.readJSON(w, r, &input)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if input.Title != nil {
		book.Title = *input.Title
	}

	if input.Author != nil {
		book.Author = *input.Author
	}

	if input.Published != nil {
		book.Published = *input.Published
	}

	if input.Pages != nil {
		book.Pages = *input.Pages
	}

	if len(input.Genres) > 0 {
		book.Genres = input.Genres
	}

	if input.Rating != nil {
		book.Rating = *input.Rating
	}

	if input.ISBN != nil {
		book.ISBN = *input.ISBN
	}

	//this is where the record is being updated in the database
	err = app.models.Books.Update(book)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	//this returns back a response of what was updated
	if err := app.writeJSON(w, http.StatusOK, envelope{"book": book}, nil); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

}

func (app *application) deleteBook(w http.ResponseWriter, r *http.Request) {
	//below is where we get access the book id from the url
	id := r.URL.Path[len("/v1/books/"):]
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		http.Error(w, "Bad Request3", http.StatusBadRequest)
	}
	fmt.Fprintf(w, "Delete the book with ID: %d", idInt)

	err = app.models.Books.Delete(idInt)
	if err != nil {
		switch {
		case errors.Is(err, errors.New("record not found")):
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		default:
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	//this is a returned response that uses the app.WriteJSON helper function that says the book was deleted
	err = app.writeJSON(w, http.StatusOK, envelope{"message": "book successfully deleted"}, nil)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

// the type below is part of making an envelope for JSON data
// this envelope type will be used to collect the JSON data within a named object which can make parsing easier
type envelope map[string]any

// Credit: Alex Edwards, Let's Go Further
// This was added to replace duplicated code in the handlers so as to observed the DRY principle
func (app *application) writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	js, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	js = append(js, '\n')

	for key, value := range headers {
		w.Header()[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)

	return nil
}

// this function replaces having an unmarshall function inside handlers.go
// it also helps protect the web service by setting a maximum allowed bytes
// and it disallows unknown fields, meaning you can pass in json fields that aren't part of the struct that is defined on the interface
// that's why this uses the decoder instead of just unmarshall
func (app *application) readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	maxBytes := 1_048_576
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes)) //sets max bytes

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() //disallows unknown fields

	if err := dec.Decode(dst); err != nil {
		return err
	}

	err := dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must only contain a single JSON object")
	}

	return nil
}
