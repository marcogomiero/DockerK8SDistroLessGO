package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/testme", testmeHandler).Methods("GET")
	r.HandleFunc("/timeout", timeoutHandler).Methods("GET")

	r.HandleFunc("/healthz", healthzHandler).Methods("GET")
	r.HandleFunc("/readyz", readyzHandler).Methods("GET")

	http.Handle("/", r)

	fmt.Println("Server avviato su :8080")
	err := http.ListenAndServe(":8080", logRequest(r))
	if err != nil {
		return
	}
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		next.ServeHTTP(w, r)
		endTime := time.Now()
		elapsed := endTime.Sub(startTime)
		log.Printf("[%s] %s %s - %v\n", r.Method, r.URL.Path, r.RemoteAddr, elapsed)
	})
}

// Ascolta su uri /testme ed accetta in ingresso
// forceHttpCode: se si vuole un RC != 200. default: 200
// delayParam: se se vuole che prima di evadere la richiesta attenda il tempo espresso in ms. default: 0
func testmeHandler(w http.ResponseWriter, r *http.Request) {
	delayParam := r.URL.Query().Get("delay")
	forceHttpCodeParam := r.URL.Query().Get("forceHttpCode")

	var delay int64
	var forceHttpCode int
	var err error

	if delayParam != "" {
		delay, err = strconv.ParseInt(delayParam, 10, 64)
		if err != nil {
			http.Error(w, "Errore nel parsing del parametro delay", http.StatusBadRequest)
			return
		}
	}

	if forceHttpCodeParam != "" {
		forceHttpCode, err = strconv.Atoi(forceHttpCodeParam)
		if err != nil {
			http.Error(w, "Errore nel parsing del parametro forceHttpCode", http.StatusBadRequest)
			return
		}
	}

	if forceHttpCode == 500 {
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		return
	}
	if forceHttpCode == 502 {
		http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
		return
	}

	xRoutedBy := r.Header.Get("x-routed-by")
	whoAmI := os.Getenv("WHO_AM_I")
	nodeName := os.Getenv("NODE_NAME")
	namespace := os.Getenv("NAMESPACE")

	createResponse(delay, forceHttpCode, xRoutedBy, whoAmI, nodeName, namespace, "testme", w)
}

func timeoutHandler(w http.ResponseWriter, _ *http.Request) {
	time.Sleep(30 * time.Second)
	http.Error(w, "504 Gateway Timeout", http.StatusGatewayTimeout)
}

// healthz probe
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	probeHandler(w, r, "healthz")
}

// readiness probe
func readyzHandler(w http.ResponseWriter, r *http.Request) {
	probeHandler(w, r, "readiness")
}

func probeHandler(w http.ResponseWriter, r *http.Request, probeType string) {
	resp, err := http.Get("http://localhost:8080/testme")
	if err != nil {
		log.Printf("Errore nella verifica di %s: %v\n", probeType, err)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, err := fmt.Fprintln(w, "Service Unavailable")
		if err != nil {
			return
		}
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	if resp.StatusCode == http.StatusOK {
		w.WriteHeader(http.StatusOK)
		_, err := fmt.Fprintln(w, "OK")
		if err != nil {
			return
		}
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, err := fmt.Fprintln(w, "Service Unavailable")
		if err != nil {
			return
		}
	}
}

func createResponse(delay int64, forceHttpCode int, xRoutedBy string, whoAmI string, nodeName string, namespace string, endpoint string, w http.ResponseWriter) {
	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	response := fmt.Sprintf(`{
  "message": "Hello World",
  "RC": "%d",
  "HEADER": "%s",
  "WHO_AM_I": "%s",
  "NODE_NAME": "%s",
  "NAMESPACE": "%s",
  "ENDPOINT": "%s"
}`, forceHttpCode, xRoutedBy, whoAmI, nodeName, namespace, endpoint)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(response))
	if err != nil {
		return
	}
}
