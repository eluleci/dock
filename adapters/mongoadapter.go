package adapters

import (
	"github.com/eluleci/dock/messages"
	"github.com/eluleci/dock/utils"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	//	"encoding/json"
	"time"
	"strings"
)

type MongoAdapter struct {
	Collection    *mgo.Collection
}

var MongoDB *mgo.Database

func (m *MongoAdapter) HandlePost(requestWrapper messages.RequestWrapper) (response map[string]interface{}, err error) {

	utils.Log("info", "mongoadapter.HandlePost")

	message := requestWrapper.Message

	objectId := bson.NewObjectId()
	createdAt := int32(time.Now().Unix())

	// additional fields
	message.Body["_id"] = objectId
	message.Body["createdAt"] = createdAt
	message.Body["updatedAt"] = createdAt

	err = m.Collection.Insert(message.Body)
	if err != nil {
		return
	}

	response = make(map[string]interface{})
	response["objectId"] = objectId
	response["createdAt"] = createdAt

	return
}

func (m *MongoAdapter) HandleGetById(requestWrapper messages.RequestWrapper) (response map[string]interface{}, err error) {

	utils.Log("info", "mongoadapter.HandleGet")

	message := requestWrapper.Message
	id := message.Res[strings.LastIndex(message.Res, "/")+1:]
	response = make(map[string]interface{})

	err = m.Collection.FindId(bson.ObjectIdHex(id)).One(&response)
	if err != nil {
		utils.Log("fatal", "Getting item with id failed")
		return
	}
	return
}

func (m *MongoAdapter) HandlePut(requestWrapper messages.RequestWrapper) (response map[string]interface{}, err error) {

	utils.Log("info", "mongoadapter.HandlePut")

	message := requestWrapper.Message
	message.Body["updatedAt"] = int32(time.Now().Unix())
	id := message.Res[strings.LastIndex(message.Res, "/") + 1:]

	objectToUpdate := make(map[string]interface{})
	err = m.Collection.FindId(bson.ObjectIdHex(id)).One(&objectToUpdate)
	if err != nil {
		utils.Log("fatal", "Getting item with id failed")
		return
	}

	// updating the fields that request contains
	for k, v := range message.Body {
		objectToUpdate[k] = v
	}

	err = m.Collection.UpdateId(bson.ObjectIdHex(id), objectToUpdate)
	if err != nil {
		utils.Log("fatal", "Updating item failed")
		return
	}

	response = make(map[string]interface{})
	response["updatedAt"] = message.Body["updatedAt"]

	return
}
