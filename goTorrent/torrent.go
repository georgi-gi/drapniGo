package torrent

import (
	"fmt"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/spec"
	"github.com/anacrolix/torrent/tracker"
	"github.com/jlabath/bitarray"
	"net"
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
	availablePieces bitarray
}

//Abstracion for a torrent
type Torrent struct {
	pieceHashes      []Hash
	trackerIP        net.IPAddr
	peers            []Peer
	downloadedPieces bitarray
	files            []TorrentFile
}

//Abstraction for a file in a torrent
type TorrentFile struct {
	//path in which to save the file
	//file size
	//file name
	//indexes of pieces for this file?
}

//Send request to the torrent tracker in order to get the peers
func (t *Torrent) GetPeers() {
	//TODO
}

func (t *Torrent) savePieceToFile( /* p Piece */ ) {
	//TODO
}

func (t *Torrent) getPiece(h Hash) /* type for piece here, error */ {
	blocks := dividePieceIntoBlocks( /*...*/ )
	for ind, val := range blocks {
		go getBlock( /*...*/ )
	}
	//wait for piece
	//return piece
}

func (t *Torrent) DownloadAll() {
	t.getPeers()
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
