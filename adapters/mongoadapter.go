package adapters

import (
	"github.com/eluleci/dock/messages"
	"github.com/eluleci/dock/utils"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	//	"encoding/json"
	"time"
	"log"
)

type MongoAdapter struct {
	Collection    *mgo.Collection
}

var MongoDB *mgo.Database

func (m *MongoAdapter) HandlePost(requestWrapper messages.RequestWrapper) (response map[string]interface{}, error *utils.Error) {

	utils.Log("info", "mongoadapter.HandlePost")

	message := requestWrapper.Message

	objectId := bson.NewObjectId()
	createdAt := int32(time.Now().Unix())

	// additional fields
	message.Body["_id"] = objectId
	message.Body["createdAt"] = createdAt
	message.Body["updatedAt"] = createdAt

	err := m.Collection.Insert(message.Body)
	if err != nil {
		log.Fatal(err)
		error = &utils.Error{500, "Creating object failed"}
		return
	}

	response = make(map[string]interface{})
	response["objectId"] = objectId
	response["createdAt"] = createdAt

	return
}
