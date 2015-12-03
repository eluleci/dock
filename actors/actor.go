package actors

import (
	"github.com/eluleci/dock/utils"
	"github.com/eluleci/dock/messages"
	"github.com/eluleci/dock/adapters"
	"github.com/eluleci/dock/auth"
	"encoding/json"
	"strings"
	"net/http"
	"fmt"
)

const (
	ActorTypeRoot = "root"
	ActorTypeCollection = "resource"
	ActorTypeObject = "object"
	ClassUsers = "users"
	ResourceTypeUsers = "/users"
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

var RootActor Actor

var CreateActor = func (res string, level int, parentInbox chan messages.RequestWrapper) (a Actor) {

	var className string
	if strings.EqualFold(res, ResourceLogin) || strings.EqualFold(res, ResourceRegister) {
		className = ClassUsers
	} else {
		className = retrieveClassName(res, level)
	}

	if level == 0 {
		a.actorType = ActorTypeRoot
	} else if level == 1 {
		a.actorType = ActorTypeCollection
	} else if level == 2 {
		a.actorType = ActorTypeObject
	}

	a.res = res
	a.level = level
	a.class = className
	a.children = make(map[string]Actor)
	a.Inbox = make(chan messages.RequestWrapper)
	a.parentInbox = parentInbox
	a.adapter = &adapters.MongoAdapter{adapters.MongoDB.C(className)}

	return
}

func retrieveClassName(res string, level int) (string) {
	if level == 0 {
		return ""
	} else if level == 1 {
		return res[1:]
	} else if level == 2 {
		return res[1:strings.LastIndex(res, "/")]
	}
	return ""

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
				// if the resource of the message is this actor's resource

				response := handleRequest(a, requestWrapper)
				a.checkAndSend(requestWrapper.Listener, response)

			} else {
				// if the resource belongs to a children actor
				childRes := getChildRes(requestWrapper.Res, a.res)

				actor, exists := a.children[childRes]
				fmt.Println(exists)

				if !exists {
					// if children doesn't exists, create a child actor for the res
					actor = CreateActor(childRes, a.level + 1, a.Inbox)
					fmt.Println(actor.res)
					go actor.Run()
					a.children[childRes] = actor
				}
				fmt.Println(actor.res)
				//   forward message to the children actor
				actor.Inbox <- requestWrapper
			}
		}
	}
}

var handleRequest = func(a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message) {

	permissions, permissionErr := auth.GetPermissions(requestWrapper, a.adapter)

	var err error
	if permissionErr.Code != 0 {
		response.Status = permissionErr.Code
	} else if strings.EqualFold(requestWrapper.Message.Command, "get") {
		if permissions["get"] || permissions["query"] {
			response, err = handleGet(a, requestWrapper)
		} else {
			response.Status = http.StatusUnauthorized
		}
	} else if strings.EqualFold(requestWrapper.Message.Command, "post") {
		if permissions["create"] {
			response, err = handlePost(a, requestWrapper)
		} else {
			response.Status = http.StatusUnauthorized
		}
	} else if strings.EqualFold(requestWrapper.Message.Command, "put") {
		if permissions["update"] {
			response, err = handlePut(a, requestWrapper)
		} else {
			response.Status = http.StatusUnauthorized
		}
	} else if strings.EqualFold(requestWrapper.Message.Command, "delete") {
		if permissions["delete"] {
			response, err = handleDelete(a, requestWrapper)
		} else {
			response.Status = http.StatusUnauthorized
		}
	}

	if err != nil && response.Status == 0 {
		response.Status = (err.(*utils.Error)).Code
		response.Body = make(map[string]interface{})
		response.Body["message"] = (err.(*utils.Error)).Message
	}
	return
}

var handleGet = func(a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message, err error) {

	if strings.EqualFold(a.res, ResourceLogin) {                        // login request
		response, err = auth.HandleLogin(requestWrapper, a.adapter)
	} else if strings.EqualFold(a.actorType, ActorTypeObject) {         // get object by id
		response.Body, err = adapters.HandleGetById(a.adapter, requestWrapper)
	} else if strings.EqualFold(a.actorType, ActorTypeCollection) {        // query objects
		response.Body, err = adapters.HandleGet(a.adapter, requestWrapper)
	}

	if err == nil {
		response.Body = filterFields(a, response.Body)
	}
	return
}

var handlePost = func(a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message, err error) {
	if strings.EqualFold(a.res, ResourceTypeUsers) {                    // post on users not allowed
		response.Status = http.StatusMethodNotAllowed
	} else if strings.EqualFold(a.res, ResourceRegister) {              // sign up request
		response, err = auth.HandleSignUp(requestWrapper, a.adapter)
	} else if strings.EqualFold(a.actorType, ActorTypeCollection) {       // create object request
		response.Body, err = adapters.HandlePost(a.adapter, requestWrapper)
		response.Status = http.StatusCreated
	} else if strings.EqualFold(a.actorType, ActorTypeObject) {         // post on objects are not allowed
		response.Status = http.StatusBadRequest
	}
	return
}

var handlePut = func(a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message, err error) {

	if strings.EqualFold(a.actorType, ActorTypeCollection) {            // put on resources are not allowed
		response.Status = http.StatusBadRequest
	} else if strings.EqualFold(a.actorType, ActorTypeObject) {        // update object
		response.Body, err = adapters.HandlePut(a.adapter, requestWrapper)
	}
	return
}

var handleDelete = func(a *Actor, requestWrapper messages.RequestWrapper) (response messages.Message, err error) {

	if strings.EqualFold(a.actorType, ActorTypeCollection) {            // delete on resources are not allowed
		response.Status = http.StatusBadRequest
	} else if strings.EqualFold(a.actorType, ActorTypeObject) {        // delete object
		response.Body, err = adapters.HandleDelete(a.adapter, requestWrapper)
		if err == nil {
			response.Status = http.StatusNoContent
		}
	}
	return
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

func filterFields(a *Actor, object map[string]interface{}) map[string]interface{} {

	// filters 'password' fields of user objects
	// TODO make a generic filter engine that takes the filter config from the configuration file such as:
	//
	//	{
	//		filterOptions: {
	//			"/users": ["password"],
	//			"/comments": ["score"]
	//		},
	//		triggersUrl: "api2.miwi.com"
	//	}
	//
	if strings.EqualFold(a.res, ResourceTypeUsers) {
		users := object["data"]
		for _, user := range users.([]map[string]interface{}) {
			delete(user, "password")
		}
	} else {
		if strings.EqualFold(a.class, ClassUsers) {
			delete(object, "password")
		}
	}

	return object
}