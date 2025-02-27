package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"readinglist/internal/data" // this imports the data package; one can use the cat go.mod command in terminal to determine how to begin import statement if needed
)

// app method handling healthcheck endpoint
func (app *Application) healthcheck(w http.ResponseWriter, r *http.Request) {
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
		"environment": app.Config.Env,
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

func (app *Application) getCreateBooksHandler(w http.ResponseWriter, r *http.Request) {
	//the if statement validates that the request at this endpoint is only either GET or POST

	//if the endpoint /v1/books is used with get, it does the following
	if r.Method == http.MethodGet {
		//The variable book defines a slice of the data type called Book
		books, err := app.Models.Books.GetAll()
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		//The code below calls the helper.go function to format, marshall, and write the json
		//the envelope that is wrapping the books variable is naming that collection of data books and then returning the data of the books variable
		if err := app.WriteJSON(w, http.StatusOK, envelope{"books": books}, nil); err != nil {
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

		err := app.ReadJSON(w, r, &input)
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

		err = app.Models.Books.Insert(book)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		//this makes the application aware of the new location for the new book
		headers := make(http.Header)                                 //this makes the new header for the http response
		headers.Set("Location", fmt.Sprintf("v1/books/%d", book.ID)) //this sets the location of the book to the value of the the books/ api with the new book's id appended to it

		//This writes the JSON response with a 201 Created status code and the Location header set
		err = app.WriteJSON(w, http.StatusCreated, envelope{"book": book}, headers)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

}

// This is another Handler - an app method handling the get, update, deleting specific books
// Below is a request multiplexer (aka a request router). It routes incoming requests to a handler using a set of rules
func (app *Application) getUpdateDeleteBooksHandler(w http.ResponseWriter, r *http.Request) {
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
func (app *Application) getBook(w http.ResponseWriter, r *http.Request) {
	//below is where we get access the book id from the url
	id := r.URL.Path[len("/v1/books/"):]
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	//this will be removed when this application si connection to a database
	//this is using the struct from the internal/data package
	book, err := app.Models.Books.Get(idInt)
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
	if err := app.WriteJSON(w, http.StatusOK, envelope{"book": book}, nil); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

}

func (app *Application) updateBook(w http.ResponseWriter, r *http.Request) {
	//below is where we get access the book id from the url
	id := r.URL.Path[len("/v1/books/"):]
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		http.Error(w, "Bad Request2", http.StatusBadRequest)
		return
	}

	book, err := app.Models.Books.Get(idInt) //this calls the database to get the specific book record with the id from the url
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
	err = app.ReadJSON(w, r, &input)
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
	err = app.Models.Books.Update(book)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	//this returns back a response of what was updated
	if err := app.WriteJSON(w, http.StatusOK, envelope{"book": book}, nil); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

}

func (app *Application) deleteBook(w http.ResponseWriter, r *http.Request) {
	//below is where we get access the book id from the url
	id := r.URL.Path[len("/v1/books/"):]
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		http.Error(w, "Bad Request3", http.StatusBadRequest)
	}
	fmt.Fprintf(w, "Delete the book with ID: %d", idInt)

	err = app.Models.Books.Delete(idInt)
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
	err = app.WriteJSON(w, http.StatusOK, envelope{"message": "book successfully deleted"}, nil)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}
