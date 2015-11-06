package actors

import (
	"github.com/eluleci/dock/utils"
	"github.com/eluleci/dock/messages"
	"github.com/eluleci/dock/adapters"
	"encoding/json"
	"strings"
	"net/http"
	"fmt"
)

const (
	ActorTypeRoot = "root"
	ActorTypeResource = "resource"
	ActorTypeObject = "object"
)

type Actor struct {
	res               string
	level             int
	actorType      	  string
	class             string
	model             map[string]interface{}
	children          map[string]Actor
	Inbox             chan messages.RequestWrapper
	parentInbox       chan messages.RequestWrapper
	adapter           *adapters.MongoAdapter
}

func CreateActor(res string, level int, parentInbox chan messages.RequestWrapper) (h Actor) {
	class := RetrieveClassName(res, level)

	if level == 0 {
		h.actorType = ActorTypeRoot
	} else if level == 1 {
		h.actorType = ActorTypeResource
	} else if level == 2 {
		h.actorType = ActorTypeObject
	}

	h.res = res
	h.level = level
	h.class = class
	h.children = make(map[string]Actor)
	h.Inbox = make(chan messages.RequestWrapper)
	h.parentInbox = parentInbox
	h.adapter = &adapters.MongoAdapter{adapters.MongoDB.C(class)}

	return
}

func RetrieveClassName(res string, level int) (string) {
	if level == 0 {
		return ""
	} else if level == 1 {
		return res[1:]
	} else if level == 2 {
		return res[1:strings.LastIndex(res, "/")]
	} else {
		return ""
	}

	// TODO: return class names of more complicated resources like: /Post/123/Author (return User)
}

func (h *Actor) Run() {
	defer func() {
		utils.Log("debug", h.res+":  Stopped running.")
	}()

	utils.Log("debug", h.res + ":  Started running as class " + h.class)

	for {
		select {
		case requestWrapper := <-h.Inbox:

			messageString, _ := json.Marshal(requestWrapper.Message)
			utils.Log("debug", h.res+": Received message: "+string(messageString))

			if requestWrapper.Res == h.res {
				// if the resource of the message is this hub's resource

				var response messages.Message
				var err error

				if strings.EqualFold(requestWrapper.Message.Command, "get") {

					if strings.EqualFold(h.actorType, ActorTypeObject) {
						response.Body, err = h.adapter.HandleGetById(requestWrapper)
						fmt.Print(response.Body)
					} else if strings.EqualFold(h.actorType, ActorTypeResource) {
						response.Body, err = h.adapter.HandleGet(requestWrapper)
					}
				} else if strings.EqualFold(requestWrapper.Message.Command, "post") {

					if strings.EqualFold(h.actorType, ActorTypeResource) {
						response.Body, err = h.adapter.HandlePost(requestWrapper)
						response.Status = http.StatusCreated
					} else if strings.EqualFold(h.actorType, ActorTypeObject) {
						response.Status = http.StatusBadRequest	// post on objects are not allowed
					}
				} else if strings.EqualFold(requestWrapper.Message.Command, "put") {

					if strings.EqualFold(h.actorType, ActorTypeResource) {
						response.Status = http.StatusBadRequest	// put on resources are not allowed
					} else if strings.EqualFold(h.actorType, ActorTypeObject) {
						response.Body, err = h.adapter.HandlePut(requestWrapper)
					}
				} else if strings.EqualFold(requestWrapper.Message.Command, "delete") {

					if strings.EqualFold(h.actorType, ActorTypeResource) {
						response.Status = http.StatusBadRequest	// delete on resources are not allowed
					} else if strings.EqualFold(h.actorType, ActorTypeObject) {
						response.Body, err = h.adapter.HandleDelete(requestWrapper)
						if err == nil {
							response.Status = http.StatusNoContent
						}
					}
				}

				if err != nil && response.Status == 0 {
					response.Status = (err.(*utils.Error)).Code
				}
				h.checkAndSend(requestWrapper.Listener, response)

			} else {
				// if the resource belongs to a children hub
				childRes := getChildRes(requestWrapper.Res, h.res)

				hub, exists := h.children[childRes]
				if !exists {
					//   if children doesn't exists, create children hub for the res
					hub = CreateActor(childRes, h.level + 1, h.Inbox)
					go hub.Run()
					h.children[childRes] = hub
				}
				//   forward message to the children hub
				hub.Inbox <- requestWrapper
			}
		}
	}
}

func (h *Actor) checkAndSend(c chan messages.Message, m messages.Message) {
	defer func() {
		if r := recover(); r != nil {
			utils.Log("debug", h.res+"Trying to send on closed channel. Removing channel from subscribers.")
			//			h.unsubscribe <- c
		}
	}()
	c <- m
}

func getChildRes(res, parentRes string) (fullPath string) {
	res = strings.Trim(res, "/")
	parentRes = strings.Trim(parentRes, "/")
	currentResSize := len(parentRes)
	resSuffix := res[currentResSize:]
	trimmedSuffix := strings.Trim(resSuffix, "/")
	directChild := strings.Split(trimmedSuffix, "/")
	relativePath := directChild[0]
	if len(parentRes) > 0 {
		fullPath = "/"+parentRes+"/"+relativePath
	} else {
		fullPath = "/"+relativePath
	}
	return
}
