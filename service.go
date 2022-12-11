package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)


var logger TransactionLogger

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

func notAllowedHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Not Allowed", http.StatusMethodNotAllowed)
}

func keyValuePutHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]
	
	value, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = Put(key, string(value))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	logger.WritePut(key, string(value))
	w.WriteHeader(http.StatusCreated)
	log.Printf("PUT key=%s value=%s\n", key, string(value))

}

func keyValueGetHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	value, err := Get(key)
	if errors.Is(err, ErrorNoSuchKey) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write([]byte(value))
	log.Printf("GET key=%s\n", key)
}

func keyValueDeleteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	err := Delete(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logger.WriteDelete(key)
	log.Printf("Delete key=%s\n", key)
}

func initializeTransactionLog() error {
	var err error

	logger, err = NewPostgresTransactionLogger(PostgresDBParams{
		host: "localhost",
		dbName: "kvs",
		user: "test",
		password: "hunter2",
	})
	if err != nil {
		return fmt.Errorf("failed to create event logger: %w", err)
	}

	events, errors := logger.ReadEvents()
	count, e, ok := 0, Event{}, true
	
	for ok && err == nil {
		select {
		case err, ok = <-errors:
		case e, ok = <-events:
			switch e.EventType {
			case EventDelete:
				err = Delete(e.Key)
				count++
			case EventPut:
				err = Put(e.Key, e.Value)
				count++
			}
		}
	}
	log.Printf("%d events replayed\n", count)
	logger.Run()
	go func() {
		for err := range logger.Err() {
			log.Print(err)
		}
	}()

	return err
}

func main() {
	err := initializeTransactionLog()
	if err != nil {
		panic(err)
	}

	r := mux.NewRouter()

	r.Use(loggingMiddleware)

	r.HandleFunc("/v1/{key}", keyValuePutHandler).Methods("PUT")
	r.HandleFunc("/v1/{key}", keyValueGetHandler).Methods("GET")
	r.HandleFunc("/v1/{key}", keyValueDeleteHandler).Methods("DELETE")


	r.HandleFunc("/v1", notAllowedHandler)
	r.HandleFunc("/v1/{key}", notAllowedHandler)

	log.Fatal(http.ListenAndServe(":8080", r))
}