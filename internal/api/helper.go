package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// the type below is part of making an envelope for JSON data
// this envelope type will be used to collect the JSON data within a named object which can make parsing easier
type envelope map[string]any

// Credit: Alex Edwards, Let's Go Further
// This was added to replace duplicated code in the handlers so as to observed the DRY principle
func (app *Application) WriteJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
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
func (app *Application) ReadJSON(w http.ResponseWriter, r *http.Request, dst any) error {
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
