package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	_ "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	fmt.Println("Server avviato su :8080")
	fmt.Println("v.0.3.5-go-stable")
	fmt.Println(`
	  ,_,   
	 {O,o}
	 /)__)
	=="="==`)

	// Read secret on startup
	secret, err := readSecret()
	if err != nil {
		fmt.Println("Errore durante la lettura del secret:", err)
	} else {

		username, usernameExists := secret["username"]
		password, passwordExists := secret["password"]

		if usernameExists && passwordExists {
			fmt.Println("Username:", username)
			fmt.Println("Password:", password)
		} else {
			fmt.Println("Chiavi mancanti nel secret.")
		}
	}

	//schedule for refresh every 5min
	go func() {
		t := time.NewTicker(300 * time.Second)

		for {
			select {
			case <-t.C:
				secret, err := readSecret()
				if err != nil {
					fmt.Println(err)
					continue
				}
				fmt.Println("Dati decodificati:", secret)

				username, usernameExists := secret["username"]
				password, passwordExists := secret["password"]

				if usernameExists && passwordExists {
					fmt.Println("Username:", username)
					fmt.Println("Password:", password)
				} else {
					fmt.Println("Chiavi mancanti nel secret.")
				}

			}
		}
	}()

	r := mux.NewRouter()
	r.HandleFunc("/testme", testmeHandler).Methods("GET")
	r.HandleFunc("/timeout", timeoutHandler).Methods("GET")

	r.HandleFunc("/healthz", healthzHandler).Methods("GET")
	r.HandleFunc("/readyz", readyzHandler).Methods("GET")

	http.Handle("/", r)

	err = http.ListenAndServe(":8080", logRequest(r))
	if err != nil {
		return
	}
}

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

	createResponse(delay, forceHttpCode, xRoutedBy, whoAmI, nodeName, w)
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

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		next.ServeHTTP(w, r)
		endTime := time.Now()
		elapsed := endTime.Sub(startTime)
		log.Printf("[%s] %s %s - %v\n", r.Method, r.URL.Path, r.RemoteAddr, elapsed)
	})
}

func readSecret() (map[string]string, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	namespace, err := getNamespace()
	if err != nil {
		return nil, err
	}
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.Background(), "my-secrets", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Decodifica i dati del secret dalla codifica base64
	decodedData := make(map[string]string)
	for key, value := range secret.Data {
		decodedValue, err := base64.StdEncoding.DecodeString(string(value))
		if err != nil {
			return nil, err
		}
		decodedData[key] = string(decodedValue)
	}
	return decodedData, nil
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

func createResponse(delay int64, forceHttpCode int, xRoutedBy string, whoAmI string, nodeName string, w http.ResponseWriter) {
	goVersion := runtime.Version()

	namespace, err := getNamespace()
	if err != nil {
		fmt.Println("Errore durante il recupero del namespace:", err)
		namespace = "unknown"
	}

	hostname, _ := os.Hostname()

	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	response := fmt.Sprintf(`{"message":"Hello World","RC":%d,"HEADER":"%s","WHO_AM_I":"%s","NODE_NAME":"%s","K8s NS":"%s","HOSTNAME":"%s","FRAMEWORK":"GoLand %s","ENDPOINT":"/testme"}`,
		forceHttpCode, xRoutedBy, whoAmI, nodeName, namespace, hostname, goVersion)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(response))
	if err != nil {
		fmt.Println("Errore durante la scrittura della risposta:", err)
		return
	}
}

func getNamespace() (string, error) {
	saPath := filepath.Join("/var/run/secrets/kubernetes.io/serviceaccount", "namespace")

	namespace, err := os.ReadFile(saPath)
	if err != nil {
		return "", err
	}

	return string(namespace), nil
}
