package proxy

import (
	"bufio"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"time"

	. "github.com/MiningPool0826/ethpool/util"
)

const (
	MaxReqSize = 1024
)

func (s *ProxyServer) ListenTCP(listenEndPoint string, timeOutStr string) {
	timeout := MustParseDuration(timeOutStr)
	s.timeout = timeout

	addr, err := net.ResolveTCPAddr("tcp", listenEndPoint)
	if err != nil {
		Error.Fatalf("Error: %v", err)
	}
	server, err := net.ListenTCP("tcp", addr)
	if err != nil {
		Error.Fatalf("Error: %v", err)
	}
	defer server.Close()

	Info.Printf("Stratum listening on %s", listenEndPoint)

	for {
		conn, err := server.AcceptTCP()
		if err != nil {
			continue
		}
		Info.Printf("Accept Stratum TCP Connection from: %s, to: %s", conn.RemoteAddr().String(), conn.LocalAddr().String())

		_ = conn.SetKeepAlive(true)

		ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

		if s.policy.IsBanned(ip) || !s.policy.ApplyLimitPolicy(ip) {
			_ = conn.Close()
			continue
		}

		cs := &Session{conn: conn, ip: ip, shareCountInv: 0, lastLocalHRSubmitTime: 0}

		s.stratumAcceptChan <- 0
		go func(cs *Session) {
			err = s.handleTCPClient(cs)
			if err != nil {
				s.removeSession(cs)
				_ = conn.Close()
			}
			<-s.stratumAcceptChan
		}(cs)
	}
}

func (s *ProxyServer) ListenTLS(listenEndPoint string, timeOutStr string, tlsCert string, tlsKey string) {
	timeout := MustParseDuration(timeOutStr)
	s.timeout = timeout

	cert, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
	if err != nil {
		Error.Fatalf("Error: %v", err)
	}

	tlsConfig := &tls.Config{}
	tlsConfig.Certificates = []tls.Certificate{cert}
	tlsConfig.Time = time.Now
	tlsConfig.Rand = rand.Reader

	server, err := tls.Listen("tcp", listenEndPoint, tlsConfig)
	if err != nil {
		Error.Fatalf("Error: %v", err)
	}
	defer server.Close()

	Info.Printf("Stratum TLS listening on %s", listenEndPoint)

	for {
		conn, err := server.Accept()
		if err != nil {
			continue
		}
		Info.Printf("Accept Stratum TCP Connection from: %s, to: %s", conn.RemoteAddr().String(), conn.LocalAddr().String())

		ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

		if s.policy.IsBanned(ip) || !s.policy.ApplyLimitPolicy(ip) {
			_ = conn.Close()
			continue
		}

		tlsConn, ok := conn.(*tls.Conn)
		if ok {
			state := tlsConn.ConnectionState()
			for _, v := range state.PeerCertificates {
				pKIXPublicKey, _ := x509.MarshalPKIXPublicKey(v.PublicKey)
				Info.Printf("x509.MarshalPKIXPublicKey: %v", pKIXPublicKey)
			}
		} else {
			_ = conn.Close()
			continue
		}

		cs := &Session{tlsConn: tlsConn, ip: ip, shareCountInv: 0, lastLocalHRSubmitTime: 0}

		s.stratumAcceptChan <- 0
		go func(cs *Session) {
			err = s.handleTLSClient(cs)
			if err != nil {
				s.removeSession(cs)
				_ = conn.Close()
			}
			<-s.stratumAcceptChan
		}(cs)
	}
}

func (s *ProxyServer) handleTCPClient(cs *Session) error {
	cs.enc = json.NewEncoder(cs.conn)
	connBuf := bufio.NewReaderSize(cs.conn, MaxReqSize)
	s.setDeadline(cs.conn)

	for {
		data, isPrefix, err := connBuf.ReadLine()
		if isPrefix {
			Error.Printf("Socket flood detected from %s", cs.ip)
			s.policy.BanClient(cs.ip)
			return err
		} else if err == io.EOF {
			Info.Printf("Client disconnected: Address: [%s] | Name: [%s] | IP: [%s]", cs.login, cs.id, cs.ip)
			s.removeSession(cs)
			_ = cs.conn.Close()
			break
		} else if err != nil {
			Error.Printf("Error reading from socket: %v | Address: [%s] | Name: [%s] | IP: [%s]", err, cs.login, cs.id, cs.ip)
			return err
		}

		if len(data) > 1 {
			var req StratumReq
			err = json.Unmarshal(data, &req)
			if err != nil {
				s.policy.ApplyMalformedPolicy(cs.ip)
				Error.Printf("handleTCPClient: Malformed stratum request from %s: %v", cs.ip, err)
				return err
			}

			// trim space character for worker
			req.Worker = strings.Trim(req.Worker, " \t\r\n")

			s.setDeadline(cs.conn)
			err = cs.handleTCPMessage(s, &req)
			if err != nil {
				Error.Printf("handleTCPMessage: %v", err)
				return err
			}
		}
	}
	return nil
}

func (s *ProxyServer) handleTLSClient(cs *Session) error {
	cs.enc = json.NewEncoder(cs.tlsConn)
	connBuf := bufio.NewReaderSize(cs.tlsConn, MaxReqSize)
	s.setTLSDeadline(cs.tlsConn)

	for {
		data, isPrefix, err := connBuf.ReadLine()
		if isPrefix {
			Error.Printf("Socket flood detected from %s", cs.ip)
			s.policy.BanClient(cs.ip)
			return err
		} else if err == io.EOF {
			Info.Printf("Client disconnected: Address: [%s] | Name: [%s] | IP: [%s]", cs.login, cs.id, cs.ip)
			s.removeSession(cs)
			_ = cs.tlsConn.Close()
			break
		} else if err != nil {
			Error.Printf("Error reading from socket: %v | Address: [%s] | Name: [%s] | IP: [%s]", err, cs.login, cs.id, cs.ip)
			return err
		}

		if len(data) > 1 {
			var req StratumReq
			err = json.Unmarshal(data, &req)
			if err != nil {
				s.policy.ApplyMalformedPolicy(cs.ip)
				Error.Printf("handleTLSClient: Malformed stratum request from %s: %v", cs.ip, err)
				return err
			}

			// trim space character for worker
			req.Worker = strings.Trim(req.Worker, " \t\r\n")

			s.setTLSDeadline(cs.tlsConn)
			err = cs.handleTCPMessage(s, &req)
			if err != nil {
				Error.Printf("handleTCPMessage: %v", err)
				return err
			}
		}
	}
	return nil
}

func (cs *Session) handleTCPMessage(s *ProxyServer, req *StratumReq) error {
	// Handle RPC methods
	switch req.Method {
	case "eth_submitLogin":
		var params []string
		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			Error.Println("Malformed stratum request (eth_submitLogin) params from", cs.ip)
			return err
		}
		reply, errReply := s.handleLoginRPC(cs, params, req.Worker)
		if errReply != nil {
			return cs.sendTCPError(req.Id, errReply)
		}
		return cs.sendTCPResult(req.Id, reply)
	case "eth_getWork":
		reply, errReply := s.handleGetWorkRPC(cs)
		if errReply != nil {
			return cs.sendTCPError(req.Id, errReply)
		}
		return cs.sendTCPResult(req.Id, &reply)
	case "eth_submitWork":
		var params []string
		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			Error.Println("Malformed stratum request (eth_submitWork) params from", cs.ip)
			return err
		}
		reply, errReply := s.handleTCPSubmitRPC(cs, req.Worker, params)
		if errReply != nil {
			return cs.sendTCPError(req.Id, errReply)
		}
		return cs.sendTCPResult(req.Id, &reply)
	case "eth_submitHashrate":
		var params []string
		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			Error.Println("Malformed stratum request (eth_submitHashrate) params from", cs.ip)
			return err
		}
		if time.Now().Unix()-cs.lastLocalHRSubmitTime > 900 {
			_, _ = s.handleTCPSubmitHashrateRPC(cs, req.Worker, params)
			cs.lastLocalHRSubmitTime = time.Now().Unix()
		}
		return cs.sendTCPResult(req.Id, true)
	default:

		errReply := s.handleUnknownRPC(cs, req.Method)
		return cs.sendTCPError(req.Id, errReply)
	}
}

func (cs *Session) sendTCPResult(id json.RawMessage, result interface{}) error {
	cs.Lock()
	defer cs.Unlock()

	message := JSONRpcResp{Id: id, Version: "2.0", Error: nil, Result: result}
	return cs.enc.Encode(&message)
}

func (cs *Session) pushNewJob(result interface{}) error {
	cs.Lock()
	defer cs.Unlock()
	// FIXME: Temporarily add ID for Claymore compliance
	message := JSONPushMessage{Version: "2.0", Result: result, Id: 0}
	return cs.enc.Encode(&message)
}

func (cs *Session) sendTCPError(id json.RawMessage, reply *ErrorReply) error {
	cs.Lock()
	defer cs.Unlock()

	message := JSONRpcResp{Id: id, Version: "2.0", Error: reply}
	err := cs.enc.Encode(&message)
	if err != nil {
		return err
	}
	return errors.New(reply.Message)
}

func (s *ProxyServer) setDeadline(conn *net.TCPConn) {
	_ = conn.SetDeadline(time.Now().Add(s.timeout))
}

func (s *ProxyServer) setTLSDeadline(tlsConn *tls.Conn) {
	_ = tlsConn.SetDeadline(time.Now().Add(s.timeout))
}

func (s *ProxyServer) registerSession(cs *Session) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	s.sessions[cs] = struct{}{}
}

func (s *ProxyServer) removeSession(cs *Session) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	delete(s.sessions, cs)
}

func (s *ProxyServer) broadcastNewJobs() {
	t := s.currentBlockTemplate()
	if t == nil || len(t.Header) == 0 || s.isSick() {
		return
	}
	reply := []string{t.Header, t.Seed, s.diff}

	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	count := len(s.sessions)
	Info.Printf("Broadcasting new job to %v stratum miners", count)

	start := time.Now()
	bcast := make(chan int, 1024)
	n := 0

	for m := range s.sessions {
		n++
		bcast <- n

		go func(cs *Session) {
			reply[2] = cs.diffNextJob

			// update session diff to diffNextJob
			cs.diff = cs.diffNextJob

			err := cs.pushNewJob(&reply)
			<-bcast
			if err != nil {
				Error.Printf("Job transmit error to %v@%v: %v", cs.login, cs.ip, err)
				s.removeSession(cs)
			} else {
				if s.config.Proxy.Tls.Enabled {
					s.setTLSDeadline(cs.tlsConn)
				} else {
					s.setDeadline(cs.conn)
				}
			}
		}(m)
	}
	Info.Printf("Jobs broadcast finished %s", time.Since(start))
}
