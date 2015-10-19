package messages

type Message struct {
	Rid                      int `json:"rid,omitempty"`
	Res                      string `json:"res,omitempty"`
	Command                  string `json:"cmd,omitempty"`
	Headers                  map[string][]string `json:"headers,omitempty"`
	Parameters               map[string][]string `json:"parameters,omitempty"`
	Body                     map[string]interface{} `json:"body,omitempty"`
	Status                   int `json:"status,omitempty"` // used only in responses
}

type RequestWrapper struct {
	Res       string
	Message   Message
	Listener  chan Message
}

type RequestError struct {
	Code    int
	Message string
	Body    map[string]interface{}
}
