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
	ActorTypeResource = "resource"
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
				// if the resource of the message is this actor's resource

				response, err := a.handleRequest(requestWrapper)

				if err != nil && response.Status == 0 {
					response.Status = (err.(*utils.Error)).Code
				}
				a.checkAndSend(requestWrapper.Listener, response)

			} else {
				// if the resource belongs to a children actor
				childRes := getChildRes(requestWrapper.Res, a.res)

				actor, exists := a.children[childRes]
				if !exists {
					// if children doesn't exists, create a child actor for the res
					actor = CreateActor(childRes, a.level + 1, a.Inbox)
					go actor.Run()
					a.children[childRes] = actor
				}
				//   forward message to the children actor
				actor.Inbox <- requestWrapper
			}
		}
	}
}

func (a *Actor) handleRequest(requestWrapper messages.RequestWrapper) (response messages.Message, err error) {

	permissions, permissionErr := auth.GetPermissions(requestWrapper, a.adapter)
	fmt.Println(permissions)

	if permissionErr.Code != 0 {
		response.Status = permissionErr.Code
	}else if strings.EqualFold(requestWrapper.Message.Command, "get") {
		if permissions["get"] || permissions["query"] {
			response, err = a.handleGet(requestWrapper)
		} else {
			err = &utils.Error{http.StatusUnauthorized, "Unauthorized"}
		}
	} else if strings.EqualFold(requestWrapper.Message.Command, "post") {
		response, err = a.handlePost(requestWrapper)
		fmt.Println(err)
	} else if strings.EqualFold(requestWrapper.Message.Command, "put") {
		response, err = a.handlePut(requestWrapper)
	} else if strings.EqualFold(requestWrapper.Message.Command, "delete") {
		response, err = a.handleDelete(requestWrapper)
	}
	return
}

func (a *Actor) isObjectTypeActor() bool {
	return strings.EqualFold(a.actorType, ActorTypeObject)
}

func (a *Actor) isResourceTypeActor() bool {
	return strings.EqualFold(a.actorType, ActorTypeResource)
}

func (a *Actor) handleGet(requestWrapper messages.RequestWrapper) (response messages.Message, err error) {

	if strings.EqualFold(a.res, ResourceLogin) {                        // login request
		response, err = auth.HandleLogin(requestWrapper, a.adapter)
	} else if strings.EqualFold(a.actorType, ActorTypeObject) {         // get object by id
		response.Body, err = a.adapter.HandleGetById(requestWrapper)

		// delete the password field if the object type is user
		if strings.EqualFold(a.class, ClassUsers) {
			delete(response.Body, "password")
		}
	} else if strings.EqualFold(a.actorType, ActorTypeResource) {        // query objects
		response.Body, err = a.adapter.HandleGet(requestWrapper)

		// TODO filter fields
		// delete the password field if the object type is user
		if strings.EqualFold(a.res, ResourceTypeUsers) {
			users := response.Body["data"]
			for _, user := range users.([]map[string]interface{}) {
				delete(user, "password")
			}
		}
	}
	return
}

func (a *Actor) handlePost(requestWrapper messages.RequestWrapper) (response messages.Message, err error) {
	if strings.EqualFold(a.res, ResourceTypeUsers) {                    // post on users not allowed
		response.Status = http.StatusMethodNotAllowed
	} else if strings.EqualFold(a.res, ResourceRegister) {              // sign up request
		response, err = auth.HandleSignUp(requestWrapper, a.adapter)
	} else if strings.EqualFold(a.actorType, ActorTypeResource) {       // create object request
		response.Body, err = a.adapter.HandlePost(requestWrapper)
		response.Status = http.StatusCreated
	} else if strings.EqualFold(a.actorType, ActorTypeObject) {         // post on objects are not allowed
		response.Status = http.StatusBadRequest
	}
	return
}

func (a *Actor) handlePut(requestWrapper messages.RequestWrapper) (response messages.Message, err error) {

	if strings.EqualFold(a.actorType, ActorTypeResource) {            // put on resources are not allowed
		response.Status = http.StatusBadRequest
	} else if strings.EqualFold(a.actorType, ActorTypeObject) {        // update object
		response.Body, err = a.adapter.HandlePut(requestWrapper)
	}
	return
}

func (a *Actor) handleDelete(requestWrapper messages.RequestWrapper) (response messages.Message, err error) {

	if strings.EqualFold(a.actorType, ActorTypeResource) {            // delete on resources are not allowed
		response.Status = http.StatusBadRequest
	} else if strings.EqualFold(a.actorType, ActorTypeObject) {        // delete object
		response.Body, err = a.adapter.HandleDelete(requestWrapper)
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
