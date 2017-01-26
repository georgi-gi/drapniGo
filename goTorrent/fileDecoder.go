package goTorrent

//Taipei torrent

import (
	"bytes"
	"crypto/sha1"
	"errors"
	//"fmt"
	bencode "github.com/jackpal/bencode-go"
	"os"
	"strings"
	//"encoding/hex"
	"io"
	//"log"
)

type FileDict struct {
	//m.Info.Files[ind].Path
	Length int64
	Path   []string
	Md5sum string
}

type InfoDict struct {
	PieceLength int64 `bencode:"piece length"`
	Pieces      string	//hashes of all pieces
	Private     int64
	Name        string
	// Single File Mode
	Length int64
	Md5sum string
	// Multiple File mode
	Files []FileDict
}

type MetaInfo struct {
	Info         InfoDict
	InfoHash     string
	Announce     string
	AnnounceList [][]string `bencode:"announce-list"`
	CreationDate string     `bencode:"creation date"`
	Comment      string
	CreatedBy    string `bencode:"created by"`
	Encoding     string
}

func getString(m map[string]interface{}, k string) string {
	if v, ok := m[k]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getSliceSliceString(m map[string]interface{}, k string) (aas [][]string) {
	if a, ok := m[k]; ok {
		if b, ok := a.([]interface{}); ok {
			for _, c := range b {
				if d, ok := c.([]interface{}); ok {
					var sliceOfStrings []string
					for _, e := range d {
						if f, ok := e.(string); ok {
							sliceOfStrings = append(sliceOfStrings, f)
						}
					}
					if len(sliceOfStrings) > 0 {
						aas = append(aas, sliceOfStrings)
					}
				}
			}
		}
	}
	return
}

func GetMetaInfo(torrent string) (metaInfo *MetaInfo, err error) {
	var input io.ReadCloser
	if input, err = os.Open(torrent); err != nil {
		return
	}

	// We need to calcuate the sha1 of the Info map, including every value in the
	// map. The easiest way to do this is to read the data using the Decode
	// API, and then pick through it manually.
	var m interface{}
	m, err = bencode.Decode(input)
	input.Close()
	if err != nil {
		err = errors.New("Couldn't parse torrent file phase 1: " + err.Error())
		return
	}

	topMap, ok := m.(map[string]interface{})
	if !ok {
		err = errors.New("Couldn't parse torrent file phase 2.")
		return
	}

	infoMap, ok := topMap["info"]
	if !ok {
		err = errors.New("Couldn't parse torrent file. info")
		return
	}
	var b bytes.Buffer
	if err = bencode.Marshal(&b, infoMap); err != nil {
		return
	}
	hash := sha1.New()
	hash.Write(b.Bytes())

	var m2 MetaInfo
	err = bencode.Unmarshal(&b, &m2.Info)
	if err != nil {
		return
	}

	m2.InfoHash = string(hash.Sum(nil))
	m2.Announce = getString(topMap, "announce")
	m2.AnnounceList = getSliceSliceString(topMap, "announce-list")
	m2.CreationDate = getString(topMap, "creation date")
	m2.Comment = getString(topMap, "comment")
	m2.CreatedBy = getString(topMap, "created by")
	m2.Encoding = strings.ToUpper(getString(topMap, "encoding"))

	metaInfo = &m2
	return
}