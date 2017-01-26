package main

import (
	"fmt"
	"github.com/georgi-gi/drapniGo/goTorrent"
)

func main() {
	testTorrent := "/home/georgi/IdeaProjects/src/github.com/georgi-gi/drapniGo/single_file.torrent"
	//m, _ := goTorrent.GetMetaInfo("/home/georgi/IdeaProjects/src/github.com/georgi-gi/drapniGo/sherlock.torrent")
	t, err := goTorrent.NewTorrentFromFile(testTorrent)
	if err != nil {
		fmt.Println(err)
	}
	//fmt.Println(t.Meta.Announce)
	t.GetPeers()
	fmt.Println(len(t.Peers))
	fmt.Println(t.Peers[0].ID)
	fmt.Println(t.Peers[0].IP)
	fmt.Println(t.Peers[0].Port)
}
