package utils

type Error struct {
	Code    int `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
