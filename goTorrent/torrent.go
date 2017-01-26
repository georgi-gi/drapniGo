package goTorrent

import (
	//"fmt"
	//"github.com/anacrolix/torrent/metainfo"
	//"github.com/anacrolix/torrent"
	//"github.com/anacrolix/torrent/tracker"
	//"github.com/jlabath/bitarray"
	"github.com/anacrolix/torrent/bencode"
	"net"
	//"net/http"
	"net/url"
	//"fmt"
	"net/http"
	"fmt"
	//"strconv"
	"strconv"
	"io/ioutil"
)

//https://wiki.theory.org/BitTorrentSpecification#request:_.3Clen.3D0013.3E.3Cid.3D6.3E.3Cindex.3E.3Cbegin.3E.3Clength.3E
//Size of the block to download from a single peer
//2^14
const BLOCK_SIZE = 16384

type Hash [20]byte

//Abstraction for a peer
type Peer struct {
	IP              net.IP
	Port            int
	ID              []byte
	//availablePieces bitarray
}

//Abstracion for a torrent
type Torrent struct {
	Meta 		 MetaInfo
	TrackerIP        net.IPAddr
	Peers            []Peer
	Interval int32 // Minimum seconds the local peer should wait before next announce.
	Leechers int32
	Seeders  int32
	//downloadedPieces bitarray
}

//Abstraction for a file in a torrent
type TorrentFile struct {
	//path in which to save the file
	//file size
	//file name
	//indexes of pieces for this file?
}

//anacrolix/torrent/tracker
type trackerHttpResponse struct {
	Interval  	int32
	FailureReason 	string
	TrackerID	string
	Incomplete	int32
	Complete	int32
	Peers		interface{}
}

func NewTorrentFromFile(file string) (*Torrent, error) {
	meta, err := GetMetaInfo(file)
	if err != nil {
		return nil, err
	}
	return &Torrent{Meta: *meta}, nil
}

func getTorrentSize(meta MetaInfo) int64 {
	if meta.Info.Files == nil {
		return meta.Info.Length
	} else {
		var size int64
		for _, file := range meta.Info.Files {
			size += file.Length
		}
		return size
	}
}

//Checks for free ports among the reserved ones for the bittorrent protocol
func getFreePortToListen() int64 {
	//TODO
	return 0
}

//Send request to the torrent tracker in order to get the peers
func (t *Torrent) GetPeers() error {
	//https://github.com/anacrolix/torrent/blob/master/tracker/tracker.go
	peerID := url.QueryEscape("gggggggggggggggggggg")
	urlencodedHash := url.QueryEscape(t.Meta.InfoHash)

	l := strconv.FormatInt(getTorrentSize(t.Meta), 10)

	req := t.Meta.Announce +
		"&info_hash=" + urlencodedHash +
		"&peer_id=" + peerID +
		"&port=6882" +
		"&downloaded=0&left=" + l +
		"&event=started"
	resp, respErr := http.Get(req)
	if respErr != nil {
		return respErr
	}
	body, bodyErr := ioutil.ReadAll(resp.Body)
	if bodyErr != nil {
		return bodyErr
	}
	resp.Body.Close()

	var trackerResp trackerHttpResponse
	err := bencode.Unmarshal(body, &trackerResp)
	if err != nil {
		return err
	}

	t.Interval = trackerResp.Interval
	t.Leechers = trackerResp.Incomplete
	t.Seeders = trackerResp.Complete
	t.Peers, _ = trackerResp.UnmarshalPeers()
	return nil
}

//anacrolix/torrent/tracker
func (t *trackerHttpResponse) UnmarshalPeers() (ret []Peer, err error) {
	switch v := t.Peers.(type) {
	/*
	case string:
		var cps []util.CompactPeer
		cps, err = util.UnmarshalIPv4CompactPeers([]byte(v))
		if err != nil {
			return
		}
		ret = make([]Peer, 0, len(cps))
		for _, cp := range cps {
			ret = append(ret, Peer{
				IP:   cp.IP[:],
				Port: int(cp.Port),
			})
		}
		return
	*/
	case []interface{}:
		for _, i := range v {
			var p Peer
			p.fromDictInterface(i.(map[string]interface{}))
			ret = append(ret, p)
		}
		return
	default:
		err = fmt.Errorf("unsupported peers value type: %T", t.Peers)
		return
	}
}

//anacrolix/torrent/tracker
func (p *Peer) fromDictInterface(d map[string]interface{}) {
	p.IP = net.ParseIP(d["ip"].(string))
	p.ID = []byte(d["peer id"].(string))
	p.Port = int(d["port"].(int64))
}

func (t *Torrent) savePieceToFile( /* p Piece */ ) {
	//TODO
}

func (t *Torrent) getPiece(h Hash) /* type for piece here, error */ {
	//blocks := dividePieceIntoBlocks( /*...*/ )
	//for ind, val := range blocks {
	//	go getBlock( /*...*/ )
	//}
	//wait for piece
	//return piece
}

func (t *Torrent) DownloadAll() {
	t.GetPeers()
	//loop here to get pieces
	//save pieces
}

func dividePieceIntoBlocks( /*...*/ ) {
	//TODO
}

func getBlock( /*...*/ ) /* representation of block here, error */ {
	sendRequestMessage( /*...*/ )
	//HERE wait for Piece message from peer
	//decode message
	//return block of data
}

func sendRequestMessage( /*...*/ ) {
	/*
			4 byte message length
		    	1 byte message id
		        payload:
			4 byte piece index (0 based)
		        4 byte block offset within the piece (in bytes)
		        4 byte block length (2^14)
	*/
}
