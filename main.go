package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	"github.com/brianvoe/gofakeit/v6"
)

var (
	port   = flag.Int("port", 8080, "listerner port")
	logger = flag.Bool("logger", false, "enable log requests")
)

var serverIsHealthy atomic.Bool

// Create structs with random injected data
type Foo struct {
	Name     string `fake:"{firstname}"`  // Any available function all lowercase
	Sentence string `fake:"{sentence:3}"` // Can call with parameters
	City     string `fake:"{city}"`
	Number   string `fake:"{number:1,10}"` // Comma separated for multiple values

	Map        map[string]int `fakesize:"2"`
	Array      []string       `fakesize:"2"`
	ArrayRange []string       `fakesize:"2,6"`
	Skip       *string        `fake:"skip"` // Set to "skip" to not generate data for
	Created    time.Time      // Can take in a fake tag as well as a format tag
}

func greet(w http.ResponseWriter, r *http.Request) {
	if serverIsHealthy.Load() {
		w.WriteHeader(http.StatusOK)

		w.Header().Set("Content-Type", "application/json")

		delayParam := r.URL.Query().Get("delay")

		if delayParam != "" {
			delay, err := time.ParseDuration(delayParam)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "Error: %s", err)
				return
			}
			time.Sleep(delay)

		}

		var foo Foo
		err := gofakeit.Struct(&foo)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error: %s", err)
			return
		}

		err = json.NewEncoder(w).Encode(foo)

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error: %s", err)
			return
		}

	} else {

		http.Error(w, "Super error", http.StatusInternalServerError)
	}

}

func fileHandler(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)

	filenName := vars["name"]

	sizeParam := r.URL.Query().Get("size")

	size, err := strconv.Atoi(sizeParam)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println("error:", err)
		size = 1
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+filenName)
	w.Header().Set("Content-Type", "image/jpeg")

	c := 10

	for i := 0; i < size; i++ {
		b := make([]byte, c)
		_, err := rand.Read(b)
		if err != nil {
			log.Println("error:", err)
			return
		}
		w.Write(b)
	}

	w.WriteHeader(http.StatusOK)

}

func simpleHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	fmt.Fprint(w, `{"name":"teste"}`)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	// Check the health of the server and return a status code accordingly
	if serverIsHealthy.Load() {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Server is healthy")
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "Server is not healthy")
	}
}

func healthFailHandler(w http.ResponseWriter, r *http.Request) {

	serverIsHealthy.Store(false)

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Server will be not healthy")
}

func healthOkHandler(w http.ResponseWriter, r *http.Request) {

	serverIsHealthy.Store(true)

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Server will be healthy")

}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s %s %s\n", r.RemoteAddr, r.Method, r.URL, r.Host, r.UserAgent())
		next.ServeHTTP(w, r)
	})
}

func main() {

	serverIsHealthy.Store(true)
	flag.Parse()

	mux := mux.NewRouter()

	mux.Use(logRequest)

	mux.HandleFunc("/healthcheck", healthHandler).Methods("GET")
	mux.HandleFunc("/healthcheck/fail", healthFailHandler).Methods("POST")
	mux.HandleFunc("/healthcheck/ok", healthOkHandler).Methods("POST")
	mux.HandleFunc("/", greet).Methods("GET")

	mux.HandleFunc("/file/{name}", fileHandler).Methods("GET")

	mux.HandleFunc("/simple", simpleHandler).Methods("GET")

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: mux,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	log.Println("Server Started")

	<-done
	log.Println("Server Stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		// extra handling here
		cancel()
	}()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v", err)
	}
	log.Println("Server Exited Properly")
}
