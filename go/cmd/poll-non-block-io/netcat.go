package main

import (
	"time"

	"github.com/Soul-Mate/netcat/pkg/util"
)

func main() {
	mesure := util.NewMesure()
	go mesure.Run(time.Millisecond * 500)
}
