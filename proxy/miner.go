package proxy

import (
	"math/big"
	"strconv"
	"strings"

	. "github.com/PowPool/ethpool/util"
	"github.com/ethereum/go-ethereum/common"
	"github.com/mutalisk999/ethash"
)

var hasher = ethash.New()

func (s *ProxyServer) processShare(login, id, ip string, shareDiff int64, t *BlockTemplate, params []string) (bool, bool) {
	nonceHex := params[0]
	hashNoNonce := params[1]
	mixDigest := params[2]
	nonce, _ := strconv.ParseUint(strings.Replace(nonceHex, "0x", "", -1), 16, 64)

	h, ok := t.headers[hashNoNonce]
	if !ok {
		Error.Printf("Stale share from %v.%v@%v", login, id, ip)
		ShareLog.Printf("Stale share from %v.%v@%v", login, id, ip)

		ms := MakeTimestamp()
		ts := ms / 1000

		err := s.backend.WriteInvalidShare(ms, ts, login, id, shareDiff)
		if err != nil {
			Error.Println("Failed to insert invalid share data into backend:", err)
		}

		return false, false
	}

	share := Block{
		number:      h.height,
		hashNoNonce: common.HexToHash(hashNoNonce),
		difficulty:  big.NewInt(shareDiff),
		nonce:       nonce,
		mixDigest:   common.HexToHash(mixDigest),
	}

	block := Block{
		number:      h.height,
		hashNoNonce: common.HexToHash(hashNoNonce),
		difficulty:  h.diff,
		nonce:       nonce,
		mixDigest:   common.HexToHash(mixDigest),
	}

	if !hasher.Verify(share) {
		ms := MakeTimestamp()
		ts := ms / 1000

		err := s.backend.WriteRejectShare(ms, ts, login, id, shareDiff)
		if err != nil {
			Error.Println("Failed to insert reject share data into backend:", err)
		}

		return false, false
	}

	if hasher.Verify(block) {
		ok, err := s.rpc().SubmitBlock(params)
		if err != nil {
			Error.Printf("Block submission failure at height %v for %v: %v", h.height, t.Header, err)
			BlockLog.Printf("Block submission failure at height %v for %v: %v", h.height, t.Header, err)
		} else if !ok {
			Error.Printf("Block rejected at height %v for %v", h.height, t.Header)
			BlockLog.Printf("Block rejected at height %v for %v", h.height, t.Header)
			return false, false
		} else {
			s.fetchBlockTemplate()
			exist, err := s.backend.WriteBlock(login, id, params, shareDiff, h.diff.Int64(), h.height, s.hashrateExpiration)
			if exist {
				ms := MakeTimestamp()
				ts := ms / 1000

				err := s.backend.WriteInvalidShare(ms, ts, login, id, shareDiff)
				if err != nil {
					Error.Println("Failed to insert invalid share data into backend:", err)
				}
				return true, false
			}
			if err != nil {
				Error.Println("Failed to insert block candidate into backend:", err)
				BlockLog.Println("Failed to insert block candidate into backend:", err)
			} else {
				Info.Printf("Inserted block %v to backend", h.height)
				BlockLog.Printf("Inserted block %v to backend", h.height)
			}
			Info.Printf("Block found by miner %v@%v at height %d", login, ip, h.height)
			BlockLog.Printf("Block found by miner %v@%v at height %d", login, ip, h.height)
		}
	} else {
		exist, err := s.backend.WriteShare(login, id, params, shareDiff, h.height, s.hashrateExpiration)
		if exist {
			ms := MakeTimestamp()
			ts := ms / 1000

			err := s.backend.WriteInvalidShare(ms, ts, login, id, shareDiff)
			if err != nil {
				Error.Println("Failed to insert invalid share data into backend:", err)
			}
			return true, false
		}
		if err != nil {
			Error.Println("Failed to insert share data into backend:", err)
		}
	}
	return false, true
}

func (s *ProxyServer) processLocalHashRate(login, id, localHRHex string) bool {
	if localHRHex[0:2] == "0x" {
		localHRHex = localHRHex[2:]
	}
	hr, err := strconv.ParseInt(localHRHex, 16, 64)
	if err != nil {
		Error.Printf("processLocalHashRate [%s]: %s", localHRHex, err.Error())
		return false
	}

	err = s.backend.WriteLocalHashRate(login, id, hr)
	if err != nil {
		Error.Println("Failed to insert local hash rate into backend:", err)
		return false
	}

	return true
}
