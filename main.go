package main

import (
	"net/http"
	//	"github.com/gorilla/mux"
	"github.com/eluleci/dock/actors"
	"github.com/eluleci/dock/adapters"
	"github.com/eluleci/dock/utils"
	"github.com/eluleci/dock/messages"
	"gopkg.in/mgo.v2"
//	"gopkg.in/mgo.v2/bson"
	"encoding/json"
	"io/ioutil"
	"strings"
	"io"
//	"fmt"
//	"log"
)

func handler(w http.ResponseWriter, r *http.Request) {

	// used for calculating time
	//	start := time.Now()

	//	vars := mux.Vars(r)
	//	res := vars["res"]
	res := r.URL.Path

	if (strings.Contains(res, ".ico")) {
		utils.Log("info", "File request.")
		// TODO: handle file requests
		return
	}

	var requestWrapper messages.RequestWrapper
	requestWrapper.Res = res
	requestWrapper.Message.Res = res
	requestWrapper.Message.Command = r.Method
	requestWrapper.Message.Headers = r.Header
	rBody, err := ioutil.ReadAll(r.Body)
	json.Unmarshal(rBody, &requestWrapper.Message.Body)

	bytes, err := json.Marshal(requestWrapper.Message)
	if err != nil {
		http.Error(w, "Internal server error", 500)
	}
	utils.Log("info", "HTTP: Received request: "+r.Method)

	responseChannel := make(chan messages.Message)
	requestWrapper.Listener = responseChannel

	actors.RootActor.Inbox <- requestWrapper

	response := <-responseChannel
	response.Status = 0    // there is no need to status in http response
	bytes, err = json.Marshal(response)
	if err != nil {
		http.Error(w, "Internal server error", 500)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	io.WriteString(w, string(bytes))

	//	elapsed := time.Since(start)
	//	util.Log("info", "HTTP: Response sent in "+elapsed.String())

	close(responseChannel)
}

func main() {
	session, err := mgo.Dial("localhost")
	if err != nil {
		panic(err)
	}
	defer session.Close()

	adapters.MongoDB = session.DB("dock-db")

	actors.RootActor = actors.CreateActor("/", 0, nil)
	go actors.RootActor.Run()

	http.HandleFunc("/", handler)
	http.ListenAndServe(":8080", nil)
}
