package modifier
import (
	"strings"
	"fmt"
	"github.com/eluleci/dock/adapters"
	"github.com/eluleci/dock/messages"
	"github.com/eluleci/dock/utils"
)

var ExpandArray = func(data map[string]interface {}, config string) (result map[string]interface{}, err *utils.Error) {

	dataArray := data["data"].([]map[string]interface {})

	resultArray := make([]map[string]interface{}, len(dataArray))
	for i, v := range dataArray {
		var expandedObject map[string]interface{}
		expandedObject, err = ExpandItem(map[string]interface {}(v), config)
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
	fmt.Println(fields)
	fmt.Println(data)

	for _, field := range fields {

		trimmedField := field
		containsChildToExpand := strings.Contains(field, "(")

		if containsChildToExpand {
			trimmedField = field[0:strings.Index(field, "(")]
		}
		fmt.Println("self: " + trimmedField)

		// expanding direct children first
		reference := data[trimmedField]
		if reference == nil {
			continue
		}
		var expandedObject map[string]interface{}
		expandedObject, err = fetchData(reference.(map[string]interface{}))
		if err != nil {
			return
		}

		// expanding children
		if containsChildToExpand {
			expandConfigOfChild := field[strings.Index(field, "(")+1:strings.LastIndex(field,")")]
			fmt.Println("childToExpand: ")
			fmt.Println(expandedObject)
			fmt.Println("expandConfigOfChild: " + expandConfigOfChild)

			var expandedChild map[string]interface{}
			expandedChild, err = ExpandItem(expandedObject, expandConfigOfChild)
			if err != nil {
				return
			}
			fmt.Println(expandedChild)
//			expandedObject[trimmedField] = expandedChild
		}

		data[trimmedField] = expandedObject
	}

	result = data
	return
}

var fetchData = func(data map[string]interface{}) (object map[string]interface{}, err *utils.Error) {
	fmt.Println("fetchData: ");
	fmt.Println(data);

	id := data["_id"].(string)
	className := data["_class"].(string)
	dbAdapter := &adapters.MongoAdapter{adapters.MongoDB.C(className)}

	var rw messages.RequestWrapper
	var m messages.Message
	m.Res = "/"+className+"/" + id
	rw.Message = m

	object, err = adapters.HandleGetById(dbAdapter, rw)
	if err != nil {
		return
	}
	fmt.Println("--------")
	fmt.Println("retrieved object:")
	fmt.Println(object)
	fmt.Println("--------")
	return
}