package main

import (
	"fmt"
	"github.com/georgi-gi/drapniGo/goTorrent"
)

func main() {
	testTorrent := "/home/georgi/IdeaProjects/src/github.com/georgi-gi/single_file.torrent"
	//vikings := "/home/georgi/IdeaProjects/src/github.com/georgi-gi/vikings_season4.torrent"
	//m, _ := goTorrent.GetMetaInfo("/home/georgi/IdeaProjects/src/github.com/georgi-gi/drapniGo/sherlock.torrent")
	t, err := goTorrent.NewTorrentFromFile(testTorrent)
	if err != nil {
		fmt.Println(err)
	}

	//l := len(t.Meta.Info.Files)

	//t.GetFilesOffsets()

	//for i := 0; i < l; i++ {
	//	fmt.Println(offsets[i])
	//	fmt.Println(paths[i])
	//}

	//fmt.Println(t.Meta.Announce)
	t.DownloadAll()
	t.Wait()
	//fmt.Println(len(t.Peers))
	//fmt.Println(t.Peers[0].ID)
	//fmt.Println(t.Peers[0].IP)
	//fmt.Println(t.Peers[0].Port)
}
