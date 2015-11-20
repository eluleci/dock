package actors

import (
	"github.com/eluleci/dock/utils"
	"github.com/eluleci/dock/messages"
	"github.com/eluleci/dock/adapters"
	"github.com/eluleci/dock/auth"
	"encoding/json"
	"strings"
	"net/http"
)

const (
	ActorTypeRoot = "root"
	ActorTypeResource = "resource"
	ActorTypeObject = "object"
	ResourceTypeUsers = "/users"
	ClassUsers = "users"
	ResourceRegister = "/register"
	ResourceLogin = "/login"
)

type Actor struct {
	res         string
	level       int
	actorType   string
	class       string
	model       map[string]interface{}
	children    map[string]Actor
	Inbox       chan messages.RequestWrapper
	parentInbox chan messages.RequestWrapper
	adapter     *adapters.MongoAdapter
}

func CreateActor(res string, level int, parentInbox chan messages.RequestWrapper) (a Actor) {
	class := RetrieveClassName(res, level)

	if strings.EqualFold(res, ResourceLogin) || strings.EqualFold(res, ResourceRegister) {
		class = ClassUsers
	}

	if level == 0 {
		a.actorType = ActorTypeRoot
	} else if level == 1 {
		a.actorType = ActorTypeResource
	} else if level == 2 {
		a.actorType = ActorTypeObject
	}

	a.res = res
	a.level = level
	a.class = class
	a.children = make(map[string]Actor)
	a.Inbox = make(chan messages.RequestWrapper)
	a.parentInbox = parentInbox
	a.adapter = &adapters.MongoAdapter{adapters.MongoDB.C(class)}

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

func (a *Actor) Run() {
	defer func() {
		utils.Log("debug", a.res + ":  Stopped running.")
	}()

	utils.Log("debug", a.res + ":  Started running as class " + a.class)

	for {
		select {
		case requestWrapper := <-a.Inbox:

			messageString, _ := json.Marshal(requestWrapper.Message)
			utils.Log("debug", a.res + ": Received message: " + string(messageString))

			if requestWrapper.Res == a.res {
				// if the resource of the message is this hub's resource

				var response messages.Message
				var err error

				if strings.EqualFold(requestWrapper.Message.Command, "get") {

					if strings.EqualFold(a.res, ResourceLogin) {                    // login request
						response, err = auth.HandleLogin(requestWrapper, a.adapter)
					} else if strings.EqualFold(a.actorType, ActorTypeObject) {        // get object by id
						response.Body, err = a.adapter.HandleGetById(requestWrapper)

						// delete the password field if the object type is user
						if strings.EqualFold(a.class, ClassUsers) {
							delete(response.Body, "password")
						}
					} else if strings.EqualFold(a.actorType, ActorTypeResource) {    // query objects
						response.Body, err = a.adapter.HandleGet(requestWrapper)

						// delete the password field if the object type is user
						if strings.EqualFold(a.res, ResourceTypeUsers) {
							users := response.Body["data"]
							for _, user := range users.([]map[string]interface{}) {
								delete(user, "password")
							}
						}
					}
				} else if strings.EqualFold(requestWrapper.Message.Command, "post") {

					if strings.EqualFold(a.res, ResourceTypeUsers) {                // post on users not allowed
						response.Status = http.StatusMethodNotAllowed
					} else if strings.EqualFold(a.res, ResourceRegister) {            // sign up request
						response, err = auth.HandleSignUp(requestWrapper, a.adapter)
					} else if strings.EqualFold(a.actorType, ActorTypeResource) {    // create object request
						response.Body, err = a.adapter.HandlePost(requestWrapper)
						response.Status = http.StatusCreated
					} else if strings.EqualFold(a.actorType, ActorTypeObject) {        // post on objects are not allowed
						response.Status = http.StatusBadRequest
					}
				} else if strings.EqualFold(requestWrapper.Message.Command, "put") {

					if strings.EqualFold(a.actorType, ActorTypeResource) {            // put on resources are not allowed
						response.Status = http.StatusBadRequest
					} else if strings.EqualFold(a.actorType, ActorTypeObject) {        // update object
						response.Body, err = a.adapter.HandlePut(requestWrapper)
					}
				} else if strings.EqualFold(requestWrapper.Message.Command, "delete") {

					if strings.EqualFold(a.actorType, ActorTypeResource) {            // delete on resources are not allowed
						response.Status = http.StatusBadRequest
					} else if strings.EqualFold(a.actorType, ActorTypeObject) {        // delete object
						response.Body, err = a.adapter.HandleDelete(requestWrapper)
						if err == nil {
							response.Status = http.StatusNoContent
						}
					}
				}

				if err != nil && response.Status == 0 {
					response.Status = (err.(*utils.Error)).Code
				}
				a.checkAndSend(requestWrapper.Listener, response)

			} else {
				// if the resource belongs to a children hub
				childRes := getChildRes(requestWrapper.Res, a.res)

				hub, exists := a.children[childRes]
				if !exists {
					//   if children doesn't exists, create children hub for the res
					hub = CreateActor(childRes, a.level + 1, a.Inbox)
					go hub.Run()
					a.children[childRes] = hub
				}
				//   forward message to the children hub
				hub.Inbox <- requestWrapper
			}
		}
	}
}

func (a *Actor) checkAndSend(c chan messages.Message, m messages.Message) {
	defer func() {
		if r := recover(); r != nil {
			utils.Log("debug", a.res + "Trying to send on closed channel.")
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
		fullPath = "/" + parentRes + "/" + relativePath
	} else {
		fullPath = "/" + relativePath
	}
	return
}
