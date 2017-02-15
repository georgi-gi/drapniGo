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
	//"strings"
	"sync"
	"bytes"
	//"strings"
	"strings"
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
	//the conn through which all the messages will be sent
	connection	*net.UDPConn
	//whether the peer is available
	//e.g. unavailable when the handshake requests were not successful
	isAvailable	bool
}

//Abstracion for a torrent
type Torrent struct {
	Meta 		 MetaInfo
	TrackerIP        net.IPAddr
	Peers            []Peer
	Interval int32 // Minimum seconds the local peer should wait before next announce.
	Leechers int32
	Seeders  int32
	MyID		 []byte
	Server		 *net.UDPConn
	//contains the indexes of the available peers from the peers array of structs
	//in order not to check every time whether the peer is available
	availPeersInds	 []int
	//downloadedPieces bitarray
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
	peersInds := make([]int, 8)
	return &Torrent{Meta: *meta, availPeersInds: peersInds}, nil
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

func (t *Torrent) getPiece(h Hash) /* type for piece here, error */ {
	//blocks := dividePieceIntoBlocks( /*...*/ )
	//for ind, val := range blocks {
	//	go getBlock( /*...*/ )
	//}
	//wait for piece
	//return piece
}

func (t *Torrent) DownloadAll() {
	err := t.makeServer()
	if err != nil {
		fmt.Println(err)
		return
	}
	go t.startListening()
	//fromAddress := t.Server.Addr().String()
	//fromAddress := t.Server.LocalAddr().String()
	fromAddress := strings.Split(t.Server.LocalAddr().String(), ":")
	port := fromAddress[len(fromAddress) - 1]
	//fmt.Println(port)
	t.getPeers(port)
	//fmt.Println(len(t.Peers))
	//fmt.Println(serverAddr.String())
	t.connectWithPeers(port)
	//loop here to get pieces
	//save pieces
}

func (t *Torrent) makeServer() (/**net.UDPAddr*/ error) {
	var err error
	serverAddr, err := net.ResolveUDPAddr("udp",":0")
	if err != nil {
		return err
	}
	t.Server, err = net.ListenUDP("udp", serverAddr)
	//t.Server, err = net.Listen("udp", ":0")
	if err != nil {
		return err
	}



	//fmt.Println("here: " + t.Server.LocalAddr().String())
	//return serverAddr, nil
	return nil
}

func (t *Torrent) startListening() {
	data := make([]byte, 1024)
	//fmt.Println("waiting on server")
	for {
		cnt, err := t.Server.Read(data)
		if err != nil && err.Error() != "EOF"{
			fmt.Println(err)
			return
		}
		if cnt > 0 {
			fmt.Println("from server: " + string(data[:]))
		}

	}
}

//Send request to the torrent tracker in order to get the peers
//DONE
func (t *Torrent) getPeers(port string) error {
	//https://github.com/anacrolix/torrent/blob/master/tracker/tracker.go
	t.MyID = []byte("-GI1111-") //450514505145
	t.MyID = append(t.MyID, []byte{4,5,0,5,1,4,5,0,5,1,4,5}...)

	peerID := url.QueryEscape(string(t.MyID[:]))
	urlencodedHash := url.QueryEscape(t.Meta.InfoHash)

	l := strconv.FormatInt(getTorrentSize(t.Meta), 10)
	req := t.Meta.Announce +
		"&info_hash=" + urlencodedHash +
		"&peer_id=" + peerID +
		"&port=" + port +
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


//send each peer a handshake message
//after the handshake, send an interested message
//wait for unchoke messages
//after all this -> ready to ask for pieces
func (t *Torrent) connectWithPeers (port string) {
	//In version 1.0 of the BitTorrent protocol, pstrlen = 19, and pstr = "BitTorrent protocol".
	var message []byte
	message = append(message, byte(19))
	message = append(message, []byte("BitTorrent protocol")...)
	message = append(message, []byte{0, 0, 0, 0, 0, 0, 0, 0}...)
	message = append(message, []byte(t.Meta.InfoHash)...)
	message = append(message, t.MyID...)
	//fmt.Println(string(message[:]))
	//fmt.Println(len(message))
	//serverAddr, err := net.ResolveUDPAddr("udp", ":" + port)

	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}
	var wg sync.WaitGroup
	cntSuccessful := 0

	sendMessages := func(peer Peer, ind int) {
		addressToListen, err := net.ResolveUDPAddr("udp",":0")
		if err != nil {
			fmt.Println(err)
			return
		}

		peer.connection, err = net.ListenUDP("udp", addressToListen)
		//t.Server, err = net.Listen("udp", ":0")
		if err != nil {
			fmt.Println(err)
			return
		}

		peerAddr, err := net.ResolveUDPAddr(
				"udp",
				peer.IP.String() + ":" + strconv.Itoa(peer.Port))
		if err != nil {
			fmt.Println(err)
			return
		}
		//fmt.Println("local:  " + peer.connection.LocalAddr().String())
		//fmt.Println("remote: " + peerAddr.String())
		//peer.connection, err = net.DialUDP(
		//	"udp", addressToListen, remoteAddr)
		//if err != nil {
		//	fmt.Println(err)
		//	//fmt.Printf("   %d\n", ind)
		//	wg.Done()
		//	return
		//}
		peer.connection.WriteToUDP(message, peerAddr)
		resp := make([]byte, 1024)
		cnt := 0
		//fmt.Println("waiting")
		for cnt == 0 {
			cnt, err = peer.connection.Read(resp)
			//_, _, err := t.Server.ReadFromUDP(resp)
			//cnt, err = peer.connection.Read(resp)
			if err != nil && err.Error() != "EOF" {
				fmt.Println(err)
				wg.Done()
				return
			}
			//fmt.Println(cnt)
		}

		fmt.Printf("%s ind: %d\n", string(resp[:]), ind)
		//if err != nil {
		//	fmt.Println(err)
		//	//fmt.Printf(" %d\n", ind)
		//	wg.Done()
		//	return
		//}

		//if bytes.Equal(resp[48:68], peer.ID) {
		//	fmt.Print(true)
		//	fmt.Printf(" %d\n", ind)
		//	cntSuccessful++
		//	wg.Done()
		//} else {
		//	fmt.Printf("not equal %d\n", ind)
		//}
		if bytes.Equal(resp[48:56], peer.ID[:]) {
			fmt.Println("equal")
		}
		wg.Done()
	}

	//wg.Add(1)
	//go sendMessages(t.Peers[3], 0)

	for ind, peer := range t.Peers {
		wg.Add(1)
		go sendMessages(peer, ind)
	}
	wg.Wait()
	fmt.Println(cntSuccessful)
	fmt.Println(len(t.Peers))
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

func (p *Peer) fromDictInterface(d map[string]interface{}) {
	p.IP = net.ParseIP(d["ip"].(string))
	p.ID = []byte(d["peer id"].(string))
	p.Port = int(d["port"].(int64))
}

func (p *Peer) handshake(message []byte) {

}

func (t *Torrent) savePieceToFile( /* p Piece */ ) {
	//TODO
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
