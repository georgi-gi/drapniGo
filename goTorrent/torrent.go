package goTorrent

import (
	"github.com/anacrolix/utp"
	"github.com/anacrolix/torrent/bencode"
	"github.com/jlabath/bitarray"
	"net"
	"os"
	"net/url"
	"net/http"
	"fmt"
	"strconv"
	"io/ioutil"
	"sync"
	"bytes"
	"strings"
	"math"
	"time"
	"encoding/binary"
)

//https://wiki.theory.org/BitTorrentSpecification#request:_.3Clen.3D0013.3E.3Cid.3D6.3E.3Cindex.3E.3Cbegin.3E.3Clength.3E
//Size of the block to download from a single peer
//2^14
const BlockSize = 16384

type hash [20]byte

//Abstraction for a peer
type peer struct {
	IP              net.IP
	Port            int
	ID              []byte
	availablePieces	*bitarray.BitArray
	//the conn through which all the messages will be sent
	connection	net.Conn
	//whether the peer is available
	//e.g. unavailable when the handshake requests were not successful
	isAvailable	bool
	isChoked	bool
	blockRespChan   chan []byte
}

//GoTorrent is the main abstraction for a torrent
type GoTorrent struct {
	Meta 		  MetaInfo
	TrackerIP         net.IPAddr
	Peers             []peer
	Interval int32 // Minimum seconds the local peer should wait before next announce.
	Leechers int32
	Seeders  int32
	MyID		  []byte
	Server		  *net.UDPConn
	//contains the indexes of the available peers from the peers array of structs
	//in order not to check every time whether the peer is available
	availPeersInds	  []int
	downloadedPieces  *bitarray.BitArray
	pendingPieces	  *bitarray.BitArray
	missingPiecesInds []int
	done 		  chan struct{}
}

//anacrolix/torrent/tracker
type trackerHTTPResponse struct {
	Interval  	int32
	FailureReason 	string
	TrackerID	string
	Incomplete	int32
	Complete	int32
	Peers		interface{}
}

//piece contains all the needed information for a piece to be saved to a file
type piece struct {
	Path 	string
	Offset 	int64
	Length	int64
	Data 	[]byte
}

//NewTorrentFromFile creates and returns a GoTorrent struct for the torrent file
func NewTorrentFromFile(file string) (*GoTorrent, error) {
	meta, err := getMetaInfo(file)
	if err != nil {
		return nil, err
	}

	return &GoTorrent{Meta: *meta}, nil
}

func getTorrentSize(meta MetaInfo) int64 {
	if meta.Info.Files == nil {
		return meta.Info.Length
	}
	var size int64
	for _, file := range meta.Info.Files {
		size += file.Length
	}
	return size
}

//DownloadAll does the whole job and contains the logic of the downloading:
//	- asks for the peers
//	- connects with them
//	- starts downloading
func (t *GoTorrent) DownloadAll() {
	//the server through which clients will ask for pieces and i will send them
	err := t.makeServer()
	if err != nil {
		fmt.Println(err)
		t.done <- struct{}{}
		return
	}
	go t.startListening()

	fromAddress := strings.Split(t.Server.LocalAddr().String(), ":")
	port := fromAddress[len(fromAddress) - 1]

	t.getPeers(port)

	err = t.connectWithPeers(port)
	if err != nil {
		fmt.Println(err)
		t.done <- struct{}{}
		return
	}

	t.missingPiecesInds = make([]int, 0)

	t.downloadFile()
	if err != nil {
		fmt.Println(err)
		t.done <- struct{}{}
		return
	}

	t.done <- struct{}{}
}

//Wait reads from a channel which means that the torrent is downloaded
func (t *GoTorrent) Wait() {
	<-t.done
}

//makeServer makes the server to listen for requests of the peers
func (t *GoTorrent) makeServer() error {
	var err error
	serverAddr, err := net.ResolveUDPAddr("udp",":0")
	if err != nil {
		return err
	}
	t.Server, err = net.ListenUDP("udp", serverAddr)
	if err != nil {
		return err
	}

	return nil
}

//startListening listens for requests from peers
//not done yet
func (t *GoTorrent) startListening() {
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

//getPeers gets the IPs and ports of all the peers from the tracker
func (t *GoTorrent) getPeers(port string) error {
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

	var trackerResp trackerHTTPResponse
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

func createHandshakeMessage(infohash string, id []byte) []byte {
	var message []byte
	message = append(message, byte(19))
	message = append(message, []byte("BitTorrent protocol")...)
	message = append(message, []byte{0, 0, 0, 0, 0, 0, 0, 0}...)
	message = append(message, []byte(infohash)...)
	message = append(message, id...)

	return message
}

func createInterestedMessage() []byte {
	return []byte{0,0,0,1,2}
}

//send each peer a handshake message
//after the handshake, send an interested message
//wait for unchoke messages
//after all this -> ready to ask for pieces
func (t *GoTorrent) connectWithPeers (port string) error {
	var mtx sync.Mutex

	//In version 1.0 of the BitTorrent protocol, pstrlen = 19, and pstr = "BitTorrent protocol".
	handshakeMessage := createHandshakeMessage(t.Meta.InfoHash, t.MyID)
	interestedMessage := createInterestedMessage()

	waitChan := make(chan struct{})

	sendMessages := func(peer peer, ind int) {
		var err error
		peer.connection, err = utp.Dial(peer.IP.String() + ":" + strconv.Itoa(peer.Port))
		if err != nil {
			return
		}

		peer.connection.Write(handshakeMessage)

		resp := make([]byte, 1024)

		timer := time.NewTimer(time.Second * 10)
		<- timer.C

		_, err = peer.connection.Read(resp)

		if err != nil && err.Error() != "EOF" {
			peer.connection.Close()
			return
		}

		if bytes.Equal(resp[48:68], peer.ID) {
			mtx.Lock()
			t.availPeersInds = append(t.availPeersInds, ind)
			mtx.Unlock()
			peer.isAvailable = true
		} else {
			peer.connection.Close()
			return
		}

		//bytes # 68,69,90,71 - message length
		//byte  # 72 - message id
		//byte  # 73-... - bitfield payload
		if resp[72] != 5 {
			fmt.Println("did not receive bitfield")
			return
		}

		bitfield := resp[73:]
		peer.fillBitfield(bitfield)

		peer.connection.Write(interestedMessage)

		err = peer.waitForUnchoke()
		if err != nil {
			fmt.Println(err)
			return
		}

		//start listening for choke of have messages from this peer
		go peer.listenForMessages()

		//send that the peer is ready and unchoked
		waitChan <- struct{}{}
		peer.blockRespChan = make(chan []byte)
	}

	for ind, peer := range t.Peers {
		go sendMessages(peer, ind)
	}
	//wait for any peer to start downloading
	select {
	case <-waitChan:
		return nil
	case <-time.After(time.Second * 20):
		return fmt.Errorf("Time-out. Cannot connect with peers.")
	}
}

//savePieceToFile saves a given piece to the file
//piece contains information about where and in which file to write it
func (t *GoTorrent) savePieceToFile(piece *piece, file *os.File) {
	file.WriteAt(piece.Data, piece.Offset)
}

//getFilesOffsets fills some of the attributes the FileDict struct for each file in the torrent
//	- set first piece index of file
//	- set last piece index of file
//	- set the first byte of the file in the first piece
//	- set the last byte of the file in the last piece
func (t *GoTorrent) getFilesOffsets() ([]int64, [][]string) {
	cntFiles := len(t.Meta.Info.Files)
	offsets := make([]int64, cntFiles)
	paths := make([][]string, cntFiles)

	var curOffset int64

	for ind, file := range t.Meta.Info.Files {
		offsets[ind] = curOffset
		curOffset += file.Length
		paths[ind] = file.Path
	}

	t.Meta.Info.Files[0].FirstPieceInd = 0
	t.Meta.Info.Files[0].FirstOffset = 0

	t.Meta.Info.Files[0].LastPieceInd = int64(math.Ceil(
		float64(t.Meta.Info.Files[0].Length) / float64(t.Meta.Info.PieceLength)))
	//last piece offset = fileLength - floor(cntPieces) * pieceLength
	t.Meta.Info.Files[0].LastByteLast = t.Meta.Info.Files[0].Length -
		(t.Meta.Info.Files[0].Length / t.Meta.Info.PieceLength) * t.Meta.Info.PieceLength

	for ind := 1; ind < cntFiles; ind++ {
		if t.Meta.Info.Files[ind-1].LastByteLast == t.Meta.Info.PieceLength {
			t.Meta.Info.Files[ind].FirstOffset = 0
			t.Meta.Info.Files[ind].FirstPieceInd = t.Meta.Info.Files[ind-1].LastPieceInd + 1
		} else {
			t.Meta.Info.Files[ind].FirstOffset = t.Meta.Info.Files[ind-1].LastByteLast + 1
			t.Meta.Info.Files[ind].FirstPieceInd = t.Meta.Info.Files[ind-1].LastPieceInd
		}

		t.Meta.Info.Files[ind].LastPieceInd = int64(math.Ceil(
			float64(offsets[ind] + t.Meta.Info.Files[ind].Length) / float64(t.Meta.Info.PieceLength)))
		//last file offset?
		t.Meta.Info.Files[ind].LastByteLast = t.Meta.Info.Files[ind].Length -
			(t.Meta.Info.Files[ind].Length / t.Meta.Info.PieceLength) * t.Meta.Info.PieceLength
	}

	//fmt.Println(t.Meta.Info.Files[0].LastPieceInd)
	//fmt.Println(t.Meta.Info.Files[0].LastByteLast)
	//fmt.Println(t.Meta.Info.Files[1].FirstPieceInd)
	//fmt.Println(t.Meta.Info.Files[1].FirstOffset)
	//fmt.Println(t.Meta.Info.Files[cntFiles-1].LastPieceInd)
	//fmt.Println(offsets[cntFiles-1] + t.Meta.Info.Files[cntFiles-1].Length)
	//fmt.Println(t.Meta.Info.Files[cntFiles-1].LastByteLast)

	//cntPieces := len(t.Meta.Info.Pieces) / 20
	//fmt.Println(cntPieces)

	return offsets, paths
}

//downloadFiles manages the downloading of all the files in a multi-file torrent by calling the downloadFile
//function for each one
func (t* GoTorrent) downloadFiles() error {
	//if len(t.Meta.Info.Files == 0) {
	//
	//}
	//
	return nil
}

//downloadFile manages downloading a single file from the torrent
//offset is the offset of the start of the file according to the torrent size
func (t *GoTorrent) downloadFile() {
	//cntPieces := len(t.Meta.Info.Pieces) / 20

	cntPeersChan := make(chan struct{}, len(t.availPeersInds))

	t.Peers[t.availPeersInds[0]].getPiece(cntPeersChan, t.Meta.Info.Pieces[0:20], 0)

	//var mtx sync.Mutex
	//
	//for ind := 0; ind < cntPieces; ind++ {
	//	cntPeersChan<-struct{}{}
	//	for _, pInd := range t.availPeersInds {
	//		if t.Peers[pInd].isAvailable && !t.Peers[pInd].isChoked && t.Peers[pInd].availablePieces.isSet(ind) {
	//			t.Peers[pInd].isAvailable = false
	//			mtx.Lock()
	//			t.pendingPieces.Set(ind)
	//			mtx.Unlock()
	//
	//			t.Peers[pInd].getPiece(cntPeersChan, []byte(t.Meta.Info.Pieces[ind*20 : ind*20+20]), ind)
	//
	//			mtx.Lock()
	//			t.pendingPieces.Unset(ind)
	//			t.downloadedPieces.Set(ind)
	//			mtx.Unlock()
	//		} else {
	//			mtx.Lock()
	//			t.missingPiecesInds = append(t.missingPiecesInds, ind)
	//			mtx.Unlock()
	//		}
	//	}
	//}
}

func (p *peer) getPiece(ch chan struct{}, pieceHash string, pieceInd int) /*piece */{
	fmt.Println("getting piece")
	defer func() {
		<-ch
		p.isAvailable = true
	}()
	bl := p.getBlock(1, 0)
	fmt.Println(bl)
}

func makeRequestMessage(pieceInd int, offset int) []byte {
	//pieceIndBytes := []byte(strconv.Itoa(pieceInd))
	//offsetBytes := []byte(strconv.Itoa(pieceInd))
	pieceIndBytes := make([]byte, 4)
	offsetBytes := make([]byte, 4)
	size := make([]byte, 4)
	binary.BigEndian.PutUint32(pieceIndBytes, uint32(pieceInd))
	binary.BigEndian.PutUint32(offsetBytes, uint32(offset))
	binary.BigEndian.PutUint32(size, 16384)

	reqMessage := make([]byte, 4)
	binary.BigEndian.PutUint32(reqMessage, 13)
	//1 byte message id
	//payload:
	//4 byte piece index (0 based)
	//4 byte block offset within the piece (in bytes)
	//4 byte block length (2^14)
	reqMessage = append(reqMessage, 6)
	reqMessage = append(reqMessage, pieceIndBytes...)
	reqMessage = append(reqMessage, offsetBytes...)
	reqMessage = append(reqMessage, size...)
	return reqMessage
}

func (p *peer) getBlock(pieceInd int, offset int) []byte {
	reqMessage := makeRequestMessage(pieceInd, offset)
	fmt.Println(reqMessage)
	p.connection.Write(reqMessage)
	fmt.Println("sent message")
	resp := make([]byte, 16397)
	resp = <- p.blockRespChan
	return resp
}

//anacrolix/torrent/tracker
func (t *trackerHTTPResponse) UnmarshalPeers() (ret []peer, err error) {
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
			var p peer
			p.fromDictInterface(i.(map[string]interface{}))
			ret = append(ret, p)
		}
		return
	default:
		err = fmt.Errorf("unsupported peers value type: %T", t.Peers)
		return
	}
}

func (p *peer) fromDictInterface(d map[string]interface{}) {
	p.IP = net.ParseIP(d["ip"].(string))
	p.ID = []byte(d["peer id"].(string))
	p.Port = int(d["port"].(int64))
}

//fillBitfield gets the bitfield from the response of the peer and fills the peer.availablePieces attribute
func (p *peer) fillBitfield(bytes []byte) {
	p.availablePieces = bitarray.New(len(bytes)*8)

	powersOfTwo := []int{1,2,4,8,16,32,64,128}
	for ind, b := range bytes {
		i := 7
		for indd, power := range powersOfTwo {
			if int(b) & power == (1 << uint(indd)) {
				p.availablePieces.Set(ind*8 + i)
			}
			i--
		}
	}
}

//waitForUnchoke waits for an unchoke message once a choke one was received or in the beginning of the connetion
func (p *peer) waitForUnchoke() error {
	resp := make([]byte, 5)
	for {
		cnt, err := p.connection.Read(resp)
		if err != nil {
			return err
		} else if cnt == 5 && resp[4] == 1{
			p.isAvailable = true
			return nil
		}
	}
}

//listenForMessages checks all the messages received from the peer
//If the message is choke, the connection will wait for an unchoked one
//If it is have, it will set the bit for the piece in the bitarray
func (p *peer) listenForMessages() {
	response := make([]byte, 16397) //messageLen + messageID + pieceInd + pieceOffset + BLOCK_SIZE
	fmt.Println("listening")
	for {
		p.connection.Read(response)
		switch response[4] {
		case 0:
			//fmt.Println("choke")
			p.isAvailable = false
			p.waitForUnchoke()
		case 4:
			fmt.Println("have")
			//NO
			ind, _ := strconv.Atoi(string(response[5:]))
			p.availablePieces.Set(ind)
		case 6:
			fmt.Println("piece message")
			p.blockRespChan <- response
		default:
			continue
		}
	}
}

func dividePieceIntoBlocks( /*...*/ ) {
	//TODO
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
