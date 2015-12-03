package main

import (
	"net/http"
//	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2"
	//	"github.com/gorilla/mux"
	"github.com/eluleci/dock/config"
	"github.com/eluleci/dock/actors"
	"github.com/eluleci/dock/adapters"
	"github.com/eluleci/dock/utils"
	"github.com/eluleci/dock/messages"
	"encoding/json"
	"io/ioutil"
	"strings"
//		"fmt"
	//	"log"
	"io"
	"os"
)

func handler(w http.ResponseWriter, r *http.Request) {

	res := r.URL.Path
	if (strings.Contains(res, ".ico")) {
		utils.Log("info", "File request.")
		// TODO: handle file requests
		return
	}

	requestWrapper, parseReqErr := parseRequest(r)
	if parseReqErr != nil {
		http.Error(w, parseReqErr.Message, parseReqErr.Code)
	}

	utils.Log("info", "HTTP: Received request: "+r.Method)

	responseChannel := make(chan messages.Message)
	requestWrapper.Listener = responseChannel

	actors.RootActor.Inbox <- requestWrapper

	response := <-responseChannel
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if response.Status != 0 {
		w.WriteHeader(response.Status)
	}

	if response.Body != nil {
		bytes, err2 := json.Marshal(response.Body)
		if err2 != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		io.WriteString(w, string(bytes))
	}


	close(responseChannel)
}

func main() {

	// reading and parsing configuration
	config, configErr := readConfig()
	if configErr != nil {
		utils.Log("fatal", configErr.Message)
		os.Exit(configErr.Code)
	}

	// creating database instance
	var dbErr *utils.Error
	adapters.MongoDB, dbErr = connectToDB(config)
	if dbErr != nil {
		utils.Log("fatal", dbErr.Message)
		os.Exit(dbErr.Code)
	}

	// creating root actor
	actors.RootActor = actors.CreateActor("/", 0, nil)
	go actors.RootActor.Run()

	http.HandleFunc("/", handler)
	http.ListenAndServe(":1707", nil)
}

func readConfig() (configuration config.Config, err *utils.Error) {

	configInBytes, configErr := ioutil.ReadFile("dock-config.json")
	if configErr == nil {
		configParseErr := json.Unmarshal(configInBytes, &configuration)
		if configParseErr != nil {
			err = &utils.Error{http.StatusInternalServerError, "Parsing dock-config.json failed."};
			return
		}
	} else {
		err = &utils.Error{http.StatusInternalServerError, "No 'dock-config.json' file found."};
		return
	}
	return
}

func connectToDB(config config.Config) (db *mgo.Database, err *utils.Error) {

	address, hasAddress := config.Mongo["address"]
	name, hasName := config.Mongo["name"]
	if  !hasAddress || !hasName {
		err = &utils.Error{http.StatusInternalServerError, "Database 'address' and 'name' must be specified in dock-config.json."};
		return
	}

	session, mongoerr := mgo.Dial(address)
	if mongoerr != nil {
		err = &utils.Error{http.StatusInternalServerError, "Database connection failed."};
		return
	}

	// TODO: find a proper way to close the session
	// defer session.Close()

	db = session.DB(name)
	return
}

func parseRequest(r *http.Request) (requestWrapper messages.RequestWrapper, err *utils.Error) {

	res := r.URL.Path
	requestWrapper.Res = res
	requestWrapper.Message.Res = res
	requestWrapper.Message.Command = r.Method
	requestWrapper.Message.Headers = r.Header
	requestWrapper.Message.Parameters = r.URL.Query()

	rBody, readErr := ioutil.ReadAll(r.Body)
	if readErr != nil {
		readErr = &utils.Error{http.StatusInternalServerError, "Parsing request body failed."};
	}
	json.Unmarshal(rBody, &requestWrapper.Message.Body)

	return
}
