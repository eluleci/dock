package actors

import (
	"github.com/eluleci/dock/utils"
	"github.com/eluleci/dock/messages"
	"github.com/eluleci/dock/adapters"
	"encoding/json"
	"strings"
)

type Actor struct {
	res               string
	class             string
	model             map[string]interface{}
	children          map[string]Actor
	Inbox             chan messages.RequestWrapper
	parentInbox       chan messages.RequestWrapper
	adapter           *adapters.MongoAdapter
}

func CreateActor(res string, parentInbox chan messages.RequestWrapper) (h Actor) {
	class := res[strings.Index(res, "/")+1:]
	h.res = res
	h.class = class
	h.children = make(map[string]Actor)
	h.Inbox = make(chan messages.RequestWrapper)
	h.parentInbox = parentInbox
	h.adapter = &adapters.MongoAdapter{adapters.MongoDB.C(class)}

	return
}

func (h *Actor) Run() {
	defer func() {
		utils.Log("debug", h.res+":  Stopped running.")
	}()

	utils.Log("debug", h.res+":  Started running.")

	for {
		select {
		case requestWrapper := <-h.Inbox:

			messageString, _ := json.Marshal(requestWrapper.Message)
			utils.Log("debug", h.res+": Received message: "+string(messageString))

			if requestWrapper.Res == h.res {
				// if the resource of the message is this hub's resource

				if strings.EqualFold(requestWrapper.Message.Command, "get") {
					h.checkAndSend(requestWrapper.Listener, requestWrapper.Message)

				} else if strings.EqualFold(requestWrapper.Message.Command, "post") {
					responseBody, err := h.adapter.HandlePost(requestWrapper)

					var answer messages.Message
					answer.Body = responseBody

					if (err != nil) {
						answer.Status = 500
					}
					h.checkAndSend(requestWrapper.Listener, answer)

				} else if strings.EqualFold(requestWrapper.Message.Command, "delete") {
					h.checkAndSend(requestWrapper.Listener, requestWrapper.Message)

				} else if strings.EqualFold(requestWrapper.Message.Command, "put") {
					h.checkAndSend(requestWrapper.Listener, requestWrapper.Message)

				}

				// if there is a subscription channel inside the request, subscribe the request sender
				// we need to subscribe the channel before we continue because there may be children hub creation
				// afterwords and we need to give all subscriptions of this hub to it's children
				//				h.addSubscription(requestWrapper)

				/*if strings.EqualFold(requestWrapper.Message.Command, "get") {

					if config.SystemConfig.PersistItemInMemory && h.model != nil {
						// if persisting in memory and if the model exists, it means we already fetched data before.
						// so return the model to listener

						response := createResponse(requestWrapper.Message.Rid, h.res, 200, h.model, "")
						h.checkAndSend(requestWrapper.Listener, response)

					} else if config.SystemConfig.PersistListInMemory && len(h.children) > 0 {
						// if persisting lists in memory and if there are children hubs, it means we have the data
						// already. so directly collect the item data from hubs and return it back
						h.returnChildListToRequest(requestWrapper)

					} else if h.adapter != nil {
						// if there is no model, and if there is adapter, get the
						// data from the adapter first.
						h.executeGetOnAdapter(requestWrapper)

					} else {
						response := createResponse(requestWrapper.Message.Rid, h.res, 501, nil, "No adapter is set for ThunderDock Server.")
						h.checkAndSend(requestWrapper.Listener, response)
					}

				} else if strings.EqualFold(requestWrapper.Message.Command, "put") {

					if h.adapter != nil {
						// if there is adapter, execute the request from adapter directly
						h.executePutOnAdapter(requestWrapper)
					} else {
						response := createResponse(requestWrapper.Message.Rid, h.res, 501, nil, "No adapter is set for ThunderDock Server.")
						h.checkAndSend(requestWrapper.Listener, response)
					}

				}  else if strings.EqualFold(requestWrapper.Message.Command, "post") {

					if h.adapter != nil {
						// it is an object creation message under this domain
						h.executePostOnAdapter(requestWrapper)
					} else {
						response := createResponse(requestWrapper.Message.Rid, h.res, 501, nil, "No adapter is set for ThunderDock Server.")
						h.checkAndSend(requestWrapper.Listener, response)
					}

				}  else if strings.EqualFold(requestWrapper.Message.Command, "delete") {

					if h.adapter != nil {
						// it is an object deletion message under this domain
						if h.executeDeleteOnAdapter(requestWrapper) {

							// removing all subscribers and notifying them that they are removed from subscriptions
							for listenerChannel, _ := range h.subscribers {
								h.removeSubscription(listenerChannel, true)
							}

							// if deletion is successful, break the loop (destroy self)
							break
						}
					} else {
						response := createResponse(requestWrapper.Message.Rid, h.res, 501, nil, "No adapter is set for ThunderDock Server.")
						h.checkAndSend(requestWrapper.Listener, response)
					}

				} else if strings.EqualFold(requestWrapper.Message.Command, "::subscribe") {

					h.addSubscription(requestWrapper)

					response := createResponse(requestWrapper.Message.Rid, h.res, 200, nil, "")
					h.checkAndSend(requestWrapper.Listener, response)

				} else if strings.EqualFold(requestWrapper.Message.Command, "::unsubscribe") {
					// removing listener from subscriptions, no need to notify the listener that it is un-subscribed
					h.removeSubscription(requestWrapper.Listener, false)

					response := createResponse(requestWrapper.Message.Rid, h.res, 200, nil, "")
					h.checkAndSend(requestWrapper.Listener, response)

					if h.checkAndDestroy() {
						// if checkAndDestroy returns true, it means we're destroying. so break the for loop to destroy
						break
					}

				} else if strings.EqualFold(requestWrapper.Message.Command, "::deleteChild") {
					// this is a message from child hub for its' deletion. when a parent hub receives this message, it
					// means that the child hub is deleted explicitly.

					childRes := requestWrapper.Message.Body["::res"].(string)
					if _, exists := h.children[childRes]; exists {

						// send broadcast message of the object deletion
						requestWrapper.Message.Command = "delete"
						requestWrapper.Message.Res = h.res
						//						go func() {
						//							h.broadcast <- requestWrapper
						//						}()
						h.broadcastMessage(requestWrapper)

						// delete the child hub
						delete(h.children, childRes)
						util.Log("debug", h.res+": Deleted child "+string(childRes))

						if h.checkAndDestroy() {
							// if checkAndDestroy returns true, it means we're destroying. so break the for loop to destroy
							break
						}
					}
				} else if strings.EqualFold(requestWrapper.Message.Command, "::destroyChild") {

					childRes := requestWrapper.Message.Body["::res"].(string)
					if _, exists := h.children[childRes]; exists {

						// delete the child hub
						delete(h.children, childRes)
						util.Log("debug", h.res+": Destroyed child "+string(childRes))

						if h.checkAndDestroy() {
							// if checkAndDestroy returns true, it means we're destroying. so break the for loop to destroy
							break
						}
					}
				} else {
					var answer message.Message
					answer.Rid = requestWrapper.Message.Rid
					answer.Res = h.res
					answer.Status = 500
					answer.Body = h.model
					h.checkAndSend(requestWrapper.Listener, answer)
				}*/

			} else {
				// if the resource belongs to a children hub
				childRes := getChildRes(requestWrapper.Res, h.res)

				hub, exists := h.children[childRes]
				if !exists {
					//   if children doesn't exists, create children hub for the res
					hub = CreateActor(childRes, h.Inbox)
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
