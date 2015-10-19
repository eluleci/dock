package utils

import (
	"math/rand"
	"fmt"
	"os"
	"log"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

var logFile *os.File

func Log(level, message string) {

	fmt.Println(message)
	go func() {

		//		if level != "info" {
		//			return
		//		}
		var err error
		logFile, err = os.OpenFile("log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			fmt.Println("error opening file: %v", err)
		}
		log.SetOutput(logFile)
		log.SetFlags(log.Lmicroseconds)
		defer logFile.Close()

		log.Println(message)
	}()
}
