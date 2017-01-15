package client

import (
	"../torrent"
	"fmt"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/spec"
	"github.com/anacrolix/torrent/tracker"
)

//A client has a single torrent for now
//Future - more than one torrent for a client
//TODO
type Client struct {
	tor torrent.Torrent
}

//Takes a full name of a torrent file and adds it to the client
//TODO
func (cl *Client) AddTorrentFromFile(fname string) (T *Torrent, err error) {
	//https://godoc.org/github.com/anacrolix/torrent/metainfo#LoadFromFile
	tMetaInfo, err = metainfo.LoadFromFile(fname)
	//TODO err handle
	spec, _ = TorrentSpecFromMetaInfo(tMetaInfo) //:t spec -> *TorrentSpec

}

func (cl *Client) DownloadTorrent(t *Torrent) {
	t.DownloadAll()
}
