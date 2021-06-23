package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
	"github.com/qawsgh/sns2ps/pkg/categories"
	"github.com/qawsgh/sns2ps/pkg/divisions"
	"github.com/qawsgh/sns2ps/pkg/entities"
	"github.com/qawsgh/sns2ps/pkg/practiscorecsv"
	"github.com/qawsgh/sns2ps/pkg/regions"
	"github.com/qawsgh/sns2ps/pkg/requests"
	"github.com/rs/cors"
)

var liveMode = getLiveMode("LIVE_MODE")

type snsinfo struct {
	MatchID     string `json:"matchid"`
	SNSUsername string `json:"snsusername"`
	SNSPassword string `json:"snspassword"`
}

const baseURL = "https://shootnscoreit.com/api/ipsc/match/"

// validateArgs validates the arguments passed to routes to ensure they are populated.
// This does not check that they are correct or valid.
func validateArgs(matchid string, username string, password string) bool {
	if matchid == "" || username == "" || password == "" {
		log.Println("[validateArgs] Returning false - error in content")
		return false
	}
	return true
}

// getMatchInfo gets information about a match
func getMatchInfo(w http.ResponseWriter, r *http.Request) {
	log.Println("[getMatchInfo] received request")
	log.Printf("[getMatchInfo] UseLocal set to %v\n", liveMode)

	var snsInfo snsinfo
	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Fprintf(w, "Kindly enter data with the matchid, username and password")
	}
	err = json.Unmarshal(reqBody, &snsInfo)
	if err != nil {
		log.Printf("[getMatchInfo] Failed in unmarshal\n")
	}
	log.Printf("[getMatchInfo] Got a request for match: %v\n", snsInfo.MatchID)

	result := validateArgs(snsInfo.MatchID, snsInfo.SNSUsername, snsInfo.SNSPassword)
	if !result {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "{\"response\": \"You need to supply your Match ID, and your Shoot 'n Score It username and password\" }")
		return
	}

	matchURL := baseURL + snsInfo.MatchID + "/"
	match, myerr := entities.Match(matchURL, snsInfo.SNSUsername, snsInfo.SNSPassword, liveMode)
	if myerr != nil {
		re := myerr.(*requests.HTTPError)
		log.Printf("[getMatchInfo] Status code: %d", re.StatusCode)
		switch re.StatusCode {
		case 401:
			w.WriteHeader(http.StatusUnauthorized)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, "{\"response\": \"Failed to login to Shoot 'n Score It - please check your username and password.\" }")
			return
		case 404:
			w.WriteHeader(http.StatusNotFound)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, "{\"response\": \"Couldn't find a match with ID %v - please check your match ID before trying again\" }", snsInfo.MatchID)
			return
		}
	}

	matchFileName := strings.ReplaceAll(match.MatchName+".csv", " ", "_")
	fmt.Println(matchFileName)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "{\"response\": \"Downloading registration for %v - this will save a file to your normal download folder named '%v'\" }", match.MatchName, matchFileName)
}

// getRegistration gets match information including squad and competitor details then
// creates a CSV file containing this information and returns it to the requestor.
func getRegistration(w http.ResponseWriter, r *http.Request) {
	log.Println("[getRegistration] received request")
	err := r.ParseForm() // Parses the request body
	if err != nil {
		log.Println("[getRegistration] Failed to get form data")
		log.Println(err)
	}
	var snsInfo snsinfo
	snsInfo.MatchID = r.Form.Get("matchid")
	snsInfo.SNSUsername = r.Form.Get("username")
	snsInfo.SNSPassword = r.Form.Get("password")

	result := validateArgs(snsInfo.MatchID, snsInfo.SNSUsername, snsInfo.SNSPassword)
	if !result {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "{\"response\": \"You need to supply your Match ID, and your Shoot 'n Score It username and password\" }")
		return
	}

	log.Printf("[getRegistration] Got a request for match: %v\n", snsInfo.MatchID)
	matchURL := baseURL + snsInfo.MatchID + "/"
	squadsURL := baseURL + snsInfo.MatchID + "/squads/"
	competitorsURL := baseURL + snsInfo.MatchID + "/competitors/"

	categories := categories.Categories()
	divisions := divisions.Divisions()
	regions := regions.Regions()

	match, _ := entities.Match(matchURL, snsInfo.SNSUsername, snsInfo.SNSPassword, liveMode)
	squads, _ := entities.Squads(squadsURL, snsInfo.SNSUsername, snsInfo.SNSPassword, liveMode)
	competitors, _ := entities.Competitors(competitorsURL, categories, divisions, *match, regions, *squads,
		snsInfo.SNSUsername, snsInfo.SNSPassword, liveMode)

	log.Printf("[getRegistration] creating CSV")
	csvContent := practiscorecsv.CSVContent(*competitors)
	csvFileName := strings.ReplaceAll(match.MatchName+".csv", " ", "_")
	b := &bytes.Buffer{}
	wr := csv.NewWriter(b)
	for line := range csvContent {
		err := wr.Write(csvContent[line])
		if err != nil {
			log.Println("[getRegistration] Failed to write csv content in HTTP response")
		}
	}
	wr.Flush()
	log.Printf("[getRegistration] sending CSV with name %v", csvFileName)
	//Send the headers before sending the file
	w.Header().Set("Content-Disposition", "attachment; filename="+csvFileName)
	w.Header().Set("Content-Type", "text/csv")

	_, writeErr := w.Write(b.Bytes())
	if writeErr != nil {
		log.Println("[getRegistration] Failed to write HTTP response")
	}
}

// healthCheck is a route that just returns a 200 and OK if the app is running
func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}

// getHTTPPort gets the http port from an environment variable, or sets it to a default
// of 8080 if the PORT env var is not set.
func getHTTPPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return ":" + port
}

// getLiveMode returns true if LIVE_MODE env var is set, otherwise false.
// This means that by default, the application will query local data instead of
// the Shoot 'n Score It website.
func getLiveMode(key string) bool {
	value := os.Getenv(key)
	return len(value) == 0
}

func main() {
	httpPort := getHTTPPort()
	if liveMode {
		log.Println("[main] Running in dummy mode - will use local dummy data")
	} else {
		log.Println("[main] Running in live mode - will query Shoot 'n Score It website")
	}

	log.Printf("[main] Starting up using port %v", httpPort)
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", healthCheck).Methods("GET")
	router.HandleFunc("/matchinfo", getMatchInfo).Methods("POST")
	router.HandleFunc("/registration", getRegistration).Methods("POST")
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
	})

	handler := c.Handler(router)
	log.Fatal(http.ListenAndServe(httpPort, handler))
}
