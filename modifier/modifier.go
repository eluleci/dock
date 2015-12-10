package modifier

import (
	"strings"
	"github.com/eluleci/dock/adapters"
	"github.com/eluleci/dock/messages"
	"github.com/eluleci/dock/utils"
	"reflect"
	"gopkg.in/mgo.v2/bson"
)

var ExpandArray = func(data map[string]interface{}, config string) (result map[string]interface{}, err *utils.Error) {

	dataArray := data["data"].([]map[string]interface{})

	resultArray := make([]map[string]interface{}, len(dataArray))
	for i, v := range dataArray {
		var expandedObject map[string]interface{}
		expandedObject, err = ExpandItem(map[string]interface{}(v), config)
		if err != nil {
			return
		}
		resultArray[i] = expandedObject
	}

	result = make(map[string]interface{})
	result["data"] = resultArray
	return
}

func ExpandItem(data map[string]interface{}, config string) (result map[string]interface{}, err *utils.Error) {

	fields := strings.Split(config, ",")

	for _, field := range fields {

		trimmedField := field
		containsChildToExpand := strings.Contains(field, "(")

		if containsChildToExpand {
			trimmedField = field[0:strings.Index(field, "(")]
		}

		reference := data[trimmedField]
		if reference == nil {
			continue
		}

		var expandedObject map[string]interface{}
		if isValidReference(reference.(map[string]interface{})) {
			expandedObject, err = fetchData(reference.(map[string]interface{}))
			if err != nil {
				return
			}
		} else {
			expandedObject = reference.(map[string]interface{})
		}

		// expanding children
		if containsChildToExpand {
			expandConfigOfChild := field[strings.Index(field, "(") + 1:strings.LastIndex(field, ")")]

			var expandedChild map[string]interface{}
			expandedChild, err = ExpandItem(expandedObject, expandConfigOfChild)
			if err != nil {
				return
			}
			_ = expandedChild
			//			expandedObject[trimmedField] = expandedChild
		}

		// TODO don't modify original object or totally modify it
		data[trimmedField] = expandedObject
	}

	result = data
	return
}

var fetchData = func(data map[string]interface{}) (object map[string]interface{}, err *utils.Error) {
	fieldType := reflect.TypeOf(data["_id"])

	var id string
	if fieldType.Kind() == reflect.String {
		id = data["_id"].(string)
	} else {
		id = data["_id"].(bson.ObjectId).Hex()
	}
	className := data["_class"].(string)
	dbAdapter := &adapters.MongoAdapter{adapters.MongoDB.C(className)}

	var rw messages.RequestWrapper
	var m messages.Message
	m.Res = "/" + className + "/" + id
	rw.Message = m

	object, err = adapters.HandleGetById(dbAdapter, rw)
	if err != nil {
		return
	}
	return
}

var isValidReference = func(data map[string]interface{}) (bool) {
	t, hasType := data["_type"]
	_, hasId := data["_id"]
	_, hasClass := data["_id"]
	return len(data) == 3 && hasType && hasId && hasClass && t == "reference"
}