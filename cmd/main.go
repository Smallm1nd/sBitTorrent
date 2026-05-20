package main

import (
	"fmt"
	"log"
	"os"

	"github.com/Smallm1nd/sBitTorrent/internal/torrentfile"
)

func main() {
	var path string
	fmt.Print("Enter the path to torrent file: ")
	_, err := fmt.Scan(&path)
	if err != nil {
		log.Fatal(err)
	}

	bencodePars, err := torrentfile.Open(path)
	if err != nil {
		log.Fatal(err)
	}

	Torrent, err := bencodePars.ToTorrent()
	if err != nil {
		log.Fatal(err)
	}

	bufFile, err := Torrent.Download()
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile(Torrent.Name, bufFile, 0666)
	if err != nil {
		log.Fatal(err)
	}

}
